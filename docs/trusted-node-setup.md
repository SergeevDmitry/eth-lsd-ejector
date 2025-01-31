# Trusted node setup for Stratis mainnet

## Create validator keys

1. Download [eth2.0-deposit-cli](https://github.com/SergeevDmitry/eth2.0-deposit-cli) tool
2. Follow instructions in README and build the tool or build it using commands:

For Linux
```
make build_linux
```

For MacOS
```
mac build_macos
```

**Note**: you need Python 3.8-3.10

3. Create validator keys following [instructions](https://github.com/SergeevDmitry/eth2.0-deposit-cli?tab=readme-ov-file#step-2-create-keys-and-deposit_data-json)

Use `0x07867c078F7a86120103A373c656Dbabd1EA58Ee` address for `--execution_address` ("withdrawal recipient address" if using prompts)
Use `0` for `--node_deposit_amount`

**Note**: the folder with validator keys will also contain `deposit_data-*.json` and `stake_data-*.json` files

## Run nodes

There are 2 options to run nodes:

1. Using [stereum](https://github.com/stratisproject/stratis-node/releases)
2. Building and running [go-stratis](https://github.com/stratisproject/go-stratis/tree/release/1.1) and [prysm-stratis](https://github.com/stratisproject/prysm-stratis/commits/release/1.1) from source

#### Import validators

If you ran nodes using stereum:

1. In the stereum app click `Staking`
2. Click `Click or drag to insert key`
3. Select keystore files that your created
4. Provide keystore password and set your wallet password

If you build and ran nodes from source:

```
validator accounts import --mainnet --keys-dir <path_to_directory_with_validator_keys> --accept-terms-of-use
```

#### Set fee recipient

If you ran nodes using stereum:

1. In the stereum app find Prysm client
2. Click gear icon
3. Click `EXPERT MODE`
4. Add `- --suggested-fee-recipient=0xAa2390899a63eE70E19061C7E034ded2fA8A1Fec` option under `command` section
5. Click `Confirm & Restart`

If you built and ran nodes from source:

1. Run `beacon-node` with `--suggested-fee-recipient=0xAa2390899a63eE70E19061C7E034ded2fA8A1Fec` option

### Run ejector

On your server with nodes:

1. Install [golang](https://go.dev/doc/install)
2. Install supervisor

```
sudo apt-get update && sudo apt-get install supervisor
```

3. Download [ejector](https://github.com/SergeevDmitry/eth-lsd-ejector) repository
4. Build and install binary

```
make install
```

5. Create supervisor `/etc/supervisor/conf.d/ejector.conf` config file with the content

```
[program:eth-lsd-ejector]
command=eth-lsd-ejector start
  --execution_endpoint=http://localhost:8545
  --consensus_endpoint=http://localhost:3500
  --keys_dir=<path_to_keystore_folder>
  --withdraw_address=0x07867c078F7a86120103A373c656Dbabd1EA58Ee
environment=KEYSTORE_PASSWORD=<keystore_password>
redirect_stderr=true
stdout_logfile=/var/log/supervisor-eth-lsd-ejector.log
stdout_logfile_maxbytes=500MB
stdout_logfile_backups=10
autostart=false
startretries=10
stopwaitsecs=60
```

Replace option values:

`<path_to_keystore_folder>` by a path to a folder containing your validator keystore files
`<keystore_password>` by a password you used to encrypt validator keystore files

**Note**: if you ran your nodes using stereum, then you probably should upload keystore files to a server separately

```
scp -r ./validator_keys <server_user>@<server_host>:/opt/
```

6. Update and restart supervisor service

```
sudo supervisorctl update && sudo supervisorctl start eth-lsd-ejector
```

## Deposit validators

1. Go to [Validator App](https://auroria.validator.stafi.stratisevm.com/tokenStake/trustDeposit/)
2. Connect your wallet

**Note**: your wallet should be registered as *Trusted Node* by an admin

3. Upload `deposit_data-*.json` file located in a folder with your validator keys
4. Click `Deposit` and approve transaction in Metamask
5. You should see list of your validators on the [Token Stake](https://validator.stafi.stratisevm.com/tokenStake/list/) page
6. Wait for some time before your validators will be validated and approved

### Stake validators

Once your validators are validated and approved and there is enough STRAX in a pool, you can stake your validators

1. On the [Token Stake](https://validator.stafi.stratisevm.com/tokenStake/list/) click *Stake* button for some validator
2. Upload `stake_data-*.json` file located in a folder with your validator keys
3. Click `Stake` and approve transaction in Metamask
4. Wait for your validator activation
