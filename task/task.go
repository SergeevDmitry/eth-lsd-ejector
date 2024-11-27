package task

import (
	"math"
	"math/big"
	"time"

	withdraw "eth-lsd-ejector/bindings/Withdraw"

	"github.com/SergeevDmitry/eth2-balance-service/shared"
	"github.com/SergeevDmitry/eth2-balance-service/shared/beacon"
	sharedTypes "github.com/SergeevDmitry/eth2-balance-service/shared/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stratisproject/prysm-stratis/beacon-chain/core/signing"
	types "github.com/stratisproject/prysm-stratis/consensus-types/primitives"
	"github.com/stratisproject/prysm-stratis/crypto/bls"
	"github.com/stratisproject/prysm-stratis/encoding/bytesutil"
	ethpb "github.com/stratisproject/prysm-stratis/proto/prysm/v1alpha1"
)

var (
	domainVoluntaryExit  = bytesutil.Uint32ToBytes4(0x04000000)
	shardCommitteePeriod = types.Epoch(256) // ShardCommitteePeriod is the minimum amount of epochs a validator must participate before exiting.
)

type Task struct {
	stop              chan struct{}
	validators        map[uint64]*Validator
	notExitValidators map[string]*Validator
	connection        *shared.Connection
	withdrawContract  *withdraw.Withdraw
	withdrawAddress   common.Address
	eth2Config        *beacon.Eth2Config
}

type Validator struct {
	ValidatorIndex uint64
	Publickey      []byte
	PrivateKey     []byte
}

func NewTask(withdrawalAddress common.Address, validators map[uint64]*Validator, notExitValidators map[string]*Validator, connection *shared.Connection) *Task {
	s := &Task{
		stop:              make(chan struct{}),
		withdrawAddress:   withdrawalAddress,
		validators:        validators,
		notExitValidators: notExitValidators,
		connection:        connection,
	}
	return s
}

func (task *Task) Start() error {

	withdrawContract, err := withdraw.NewWithdraw(task.withdrawAddress, task.connection.Eth1Client())
	if err != nil {
		return err
	}
	task.withdrawContract = withdrawContract

	ethConfig, err := task.connection.Eth2Client().GetEth2Config()
	if err != nil {
		return err
	}
	task.eth2Config = &ethConfig

	SafeGoWithRestart(task.monitorHandler)
	return nil
}

func (task *Task) Stop() {
	close(task.stop)
}

func (task *Task) monitorHandler() {
	logrus.Info("start monitor")

	for {
		select {
		case <-task.stop:
			logrus.Info("task has stopped")
			return
		default:
			startCycle, err := task.withdrawContract.EjectedStartCycle(task.connection.CallOpts(nil))
			if err != nil {
				logrus.Warnf("monitor err: %s", err)
				time.Sleep(6 * time.Second)
				continue
			}

			currentCycle, err := task.withdrawContract.CurrentWithdrawCycle(task.connection.CallOpts(nil))
			if err != nil {
				logrus.Warnf("monitor err: %s", err)
				time.Sleep(6 * time.Second)
				continue
			}
			logrus.Debugf("startCycle: %d, currentCycle: %d", startCycle.Uint64(), currentCycle.Uint64())

			start := startCycle.Int64()
			end := currentCycle.Int64()
			if start == 0 {
				start = end - 20
			}
			for i := start; i <= end; {
				err := task.checkCycle(i)
				if err != nil {
					logrus.Warnf("monitor check cycle: %d err: %s", i, err)
					time.Sleep(6 * time.Second)
					continue
				}
				i++
			}
		}

		break
	}

	for {
		select {
		case <-task.stop:
			logrus.Info("task has stopped")
			return
		default:

			logrus.Debug("checkCycle start -----------")
			currentCycle, err := task.withdrawContract.CurrentWithdrawCycle(task.connection.CallOpts(nil))
			if err != nil {
				logrus.Warnf("get currentWithdrawCycle err: %s", err)
				time.Sleep(6 * time.Second)
				continue
			}

			start := currentCycle.Int64() - 10
			end := currentCycle.Int64()

			for i := start; i <= end; i++ {
				err = task.checkCycle(i)
				if err != nil {
					logrus.Warnf("checkCycle %d err: %s", i, err)
					time.Sleep(6 * time.Second)
					continue
				}
			}
			logrus.Debug("checkCycle end -----------")

			logrus.Debug("checkNotExitPubkey start -----------")
			err = task.checkNotExitPubkey()
			if err != nil {
				logrus.Warnf("checkNotExitPubkey err: %s", err)
				time.Sleep(6 * time.Second)
				continue
			}

			logrus.Debug("checkNotExitPubkey end -----------")

		}

		time.Sleep(60 * time.Second)
	}
}

func (task *Task) checkCycle(cycle int64) error {
	logrus.Debugf("checkCycle %d", cycle)
	ejectedValidators, err := task.withdrawContract.GetEjectedValidatorsAtCycle(task.connection.CallOpts(nil), big.NewInt(cycle))
	if err != nil {
		return err
	}

	for _, ejectedValidator := range ejectedValidators {
		if validator, exist := task.validators[ejectedValidator.Uint64()]; exist {
			logrus.Infof("validator %d elected at cycle %d", validator.ValidatorIndex, cycle)
			// check beacon sync status
			syncStatus, err := task.connection.Eth2Client().GetSyncStatus()
			if err != nil {
				return err
			}
			if syncStatus.Syncing {
				return errors.New("could not perform exit: beacon node is syncing.")
			}
			beaconHead, err := task.connection.Eth2Client().GetBeaconHead()
			if err != nil {
				return err
			}
			// check exited before
			pubkey := sharedTypes.BytesToValidatorPubkey(validator.Publickey)
			status, err := task.connection.GetValidatorStatus(pubkey, &beacon.ValidatorStatusOptions{Epoch: &beaconHead.Epoch})
			if err != nil {
				return err
			}
			// will skip if already sign exit
			if status.ExitEpoch != math.MaxUint64 {
				logrus.Infof("validator %d will exit at epoch %d", validator.ValidatorIndex, status.ExitEpoch)
				delete(task.validators, status.Index)
				continue
			}

			currentEpoch := types.Epoch(beaconHead.Epoch)

			// not active
			if uint64(currentEpoch) < status.ActivationEpoch {
				logrus.Warnf("validator %d is not active and can't exit, will skip, active epoch: %d current epoch: %d", validator.ValidatorIndex, status.ActivationEpoch, currentEpoch)
				continue
			}
			if currentEpoch < types.Epoch(status.ActivationEpoch)+shardCommitteePeriod {
				logrus.Warnf("validator %d is not active long enough and can't exit, will skip, active epoch: %d current epoch: %d", validator.ValidatorIndex, status.ActivationEpoch, currentEpoch)
				continue
			}

			// will sign and broadcast exit msg
			exit := &ethpb.VoluntaryExit{Epoch: currentEpoch, ValidatorIndex: types.ValidatorIndex(validator.ValidatorIndex)}

			// Use Cappella fork as here - https://github.com/stratisproject/prysm-stratis/blob/develop/beacon-chain/core/blocks/exit.go#L122
			fork := &ethpb.Fork{
				PreviousVersion: task.eth2Config.CapellaForkVersion,
				CurrentVersion:  task.eth2Config.CapellaForkVersion,
				Epoch:           types.Epoch(task.eth2Config.CapellaForkEpoch),
			}
			domain, err := signing.Domain(fork, exit.Epoch, domainVoluntaryExit, task.eth2Config.GenesisValidatorsRoot)
			if err != nil {
				return errors.Wrap(err, "Get domainData failed")
			}
			exitRoot, err := signing.ComputeSigningRoot(exit, domain)
			if err != nil {
				return errors.Wrap(err, "ComputeSigningRoot failed")
			}

			secretKey, err := bls.SecretKeyFromBytes(validator.PrivateKey)
			if err != nil {
				return errors.Wrap(err, "failed to initialize keys caches from account keystore")
			}
			sig := secretKey.Sign(exitRoot[:])

			err = task.connection.Eth2Client().ExitValidator(validator.ValidatorIndex, uint64(currentEpoch), sharedTypes.BytesToValidatorSignature(sig.Marshal()))
			if err != nil {
				return err
			}

			logrus.Infof("validator %d broadcast voluntary exit ok", validator.ValidatorIndex)

		}
	}
	return nil
}

func (task *Task) checkNotExitPubkey() error {
	for key, val := range task.notExitValidators {
		beaconHead, err := task.connection.Eth2Client().GetBeaconHead()
		if err != nil {
			return err
		}
		pubkey, err := sharedTypes.HexToValidatorPubkey(key)
		if err != nil {
			return err
		}
		status, err := task.connection.GetValidatorStatus(pubkey, &beacon.ValidatorStatusOptions{Epoch: &beaconHead.Epoch})
		if err != nil {
			return err
		}
		// will skip if already sign exit
		if status.Exists {
			val.ValidatorIndex = status.Index
			task.validators[status.Index] = val

			delete(task.notExitValidators, key)
		}
	}
	return nil
}
