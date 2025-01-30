
# Infrastructure Setup for Liquid Staking

This guide outlines the steps to set up the infrastructure required for our liquid staking solution. It covers installing system dependencies, setting up Python 3.10 via PyEnv, installing Go 1.23.5, cloning and building necessary repositories, and configuring Supervisor to manage services.

---

## Table of Contents

1. [System Dependencies](#system-dependencies)  
2. [Install PyEnv and Python 3.10](#install-pyenv-and-python-310)  
3. [Install Go 1.23.5](#install-go-1235)  
4. [Clone Repositories](#clone-repositories)  
5. [Build and Install Components](#build-and-install-components)  
   1. [deposit-cli](#build-deposit-cli)  
   2. [eth-lsd-ejector](#build-ejector)  
   3. [eth-lsd-relay](#build-relay)  
6. [Generate Validator Keys](#generate-validator-keys)  
7. [Configure and Run Services](#configure-and-run-services)  
   1. [Configure Ejector](#configure-ejector)  
   2. [Configure Relay](#configure-relay)  
   3. [Import Private Key for Relay](#import-private-key-for-relay)  
   4. [Supervisor Config for Voter](#supervisor-config-for-voter)  
8. [Funding & Running Services](#funding--running-services)  
9. [Notes](#notes)

---

## System Dependencies

Update and upgrade existing packages, then install the required dependencies:

    apt update && apt upgrade -y
    apt install -y git build-essential libssl-dev zlib1g-dev libbz2-dev \
      libreadline-dev libsqlite3-dev wget curl llvm libncurses5-dev \
      libncursesw5-dev xz-utils tk-dev libffi-dev liblzma-dev \
      software-properties-common supervisor

---

## Install PyEnv and Python 3.10

1. **Clone pyenv** into `~/.pyenv`:

        git clone https://github.com/pyenv/pyenv.git ~/.pyenv

2. **Configure environment variables** (add these lines to `~/.bashrc` or `~/.zshrc`):

        export PYENV_ROOT="$HOME/.pyenv"
        export PATH="$PYENV_ROOT/bin:$PATH"
        eval "$(pyenv init --path)"
        eval "$(pyenv init -)"

3. **Reload your shell**:

        source ~/.bashrc
    or

        source ~/.zshrc

4. **Install Python 3.10** using pyenv and set it globally:

        pyenv install 3.10
        pyenv global 3.10

---

## Install Go 1.23.5

Download, extract, and configure Go:

    wget https://go.dev/dl/go1.23.5.linux-amd64.tar.gz
    rm -rf /usr/local/go
    tar -C /usr/local -xzf go1.23.5.linux-amd64.tar.gz
    export PATH=$PATH:/usr/local/go/bin
    source ~/.bashrc

> **Note**: Ensure that `/usr/local/go/bin` is added to your `PATH` in your shell configuration file (e.g., `~/.bashrc` or `~/.zshrc`).

---

## Clone Repositories

    git clone https://github.com/SergeevDmitry/eth2.0-deposit-cli
    git clone https://github.com/SergeevDmitry/eth-lsd-ejector
    git clone https://github.com/SergeevDmitry/eth-lsd-relay

---

## Build and Install Components

### Build deposit-cli

    cd eth2.0-deposit-cli
    make build_linux
    ./deposit.sh install

### Build Ejector

    cd ../eth-lsd-ejector/
    make install

### Build Relay

    cd ../eth-lsd-relay/
    make build-linux
    cd ..

---

## Generate Validator Keys

1. Navigate to the `deposit-cli` directory:

        cd eth2.0-deposit-cli

2. Run the key generation script:

        ./deposit.sh new-mnemonic

You will be prompted for:

- **Number of validators** (e.g., `100`)  
- **Amount a validator must deposit** (type `0` for trust node)  
- **Withdrawal recipient address** (`0x07867c078F7a86120103A373c656Dbabd1EA58Ee`)  
- **Network/chain name** (e.g., `stratis` or `auroria`)  
- **Password** to secure your validator keystore(s)

Keep a note of the mnemonic phrase and the password. **Store these securely.**

The script will generate keys in the `validator_keys` directory, along with JSON files `deposit_data` and `stake_data`.

> **Next**: You can load these validator keys into your validator using the Stratis Launcher.

---

## Configure and Run Services

### Configure Ejector

Create an Ejector configuration under Supervisor by editing (or creating) `/etc/supervisor/conf.d/ejector.conf`:

    vi /etc/supervisor/conf.d/ejector.conf

Paste in the following snippet (update `<keystore_password>` to match the password used to create your validator keys):

    [program:eth-lsd-ejector]
    command=eth-lsd-ejector start \
      --execution_endpoint=http://localhost:8545 \
      --consensus_endpoint=http://localhost:3500 \
      --keys_dir=/root/eth2.0-deposit-cli/validator_keys \
      --withdraw_address=0x07867c078F7a86120103A373c656Dbabd1EA58Ee
    environment=KEYSTORE_PASSWORD=<keystore_password>
    redirect_stderr=true
    stdout_logfile=/var/log/supervisor-eth-lsd-ejector.log
    stdout_logfile_maxbytes=500MB
    stdout_logfile_backups=10
    autostart=false
    startretries=10
    stopwaitsecs=60

### Configure Relay

1. Generate a new Ethereum account (using MetaMask, etc.) and **export the private key**.

2. Create a new config file for the relay at `/opt/voter/config.toml`:

    `vi /opt/voter/config.toml`

Use the snippet below, replacing `<your_account>` with your newly created address:

    account = "<your_account>"
    gasLimit = "3000000"
    maxGasPrice = "60000000000"       # wei
    trustNodeDepositAmount = 1        # ether
    eth2EffectiveBalance = 20000      # ether
    batchRequestBlocksNumber = 32
    runForEntrustedLsdNetwork = false

    [pinata]
    apikey = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySW5mb3JtYXRpb24iOnsiaWQiOiI2ZTAyNGJlMS1mODQ4LTRiYmEtYjRlZS1hZWVhMGQ2MjNjNDgiLCJlbWFpbCI6ImNvcnBvcmF0ZUBzdHJhdGlzcGxhdGZvcm0uY29tIiwiZW1haWxfdmVyaWZpZWQiOnRydWUsInBpbl9wb2xpY3kiOnsicmVnaW9ucyI6W3siZGVzaXJlZFJlcGxpY2F0aW9uQ291bnQiOjEsImlkIjoiRlJBMSJ9LHsiZGVzaXJlZFJlcGxpY2F0aW9uQ291bnQiOjEsImlkIjoiTllDMSJ9XSwidmVyc2lvbiI6MX0sIm1mYV9lbmFibGVkIjpmYWxzZSwic3RhdHVzIjoiQUNUSVZFIn0sImF1dGhlbnRpY2F0aW9uVHlwZSI6InNjb3BlZEtleSIsInNjb3BlZEtleUtleSI6IjdkZTUwMDUzZDQwMzZlYjk1YTljIiwic2NvcGVkS2V5U2VjcmV0IjoiNTE3YmU5MDQ3YWIzNWU3YmU2YjlmOGM5OWJhMjAzN2E3MmU2Yzg0MGM4ZWI4N2MxNmY4MGNlODk5MzUzY2JiNiIsImV4cCI6MTc2MzY0ODUyNX0.Qp0QvflAg1yWIDHVzUmKZrMW0AivN7QWJAV_t_27xv4"
    pinDays = 180

    [contracts]
    lsdTokenAddress = "0x2CaE3B96F7A202aE4460A391b9b4eA35dD606931"
    lsdFactoryAddress = "0x68443F30c4F347C6636775d8ffAEE3b6428E1d0d"

    [[endpoints]]
    eth1 = "https://rpc.stratisevm.com"
    eth2 = "https://rpc.stratisevm.com/prysm"

### Import Private Key for Relay

    eth-lsd-relay import-account --base-path /opt/voter

Follow the prompts to import your private key.

### Supervisor Config for Voter

Create a Supervisor configuration at `/etc/supervisor/config.d/voter.conf`:

    vi /etc/supervisor/config.d/voter.conf

Paste:

    [program:voter]
    command=eth-lsd-relay start --base-path /opt/voter
    environment=KEYSTORE_PASSWORD=<keystore_password>
    redirect_stderr=true
    stdout_logfile=/var/log/supervisor-voter.log
    stdout_logfile_maxbytes=500MB
    stdout_logfile_backups=10
    autostart=true
    startretries=10
    stopwaitsecs=60

---

## Funding & Running Services

1. **Deposit STRAX**:  
   You should deposit **at least 10 STRAX** to the new account you created for relay.

2. **Update & Start Supervisor**:

       supervisorctl update
       supervisorctl start voter
       supervisorctl start eth-lsd-ejector

---

## Notes

- **Validator Keys**: Keep your mnemonic phrase and keystore password safe.  
- **Ejector & Relay Logs**: Supervisor writes logs to `/var/log/supervisor-*.log`. Check these with the below commands:
>     tail -f /var/log/supervisor-voter.log
>     tail -f /var/log/supervisor-eth-lsd-ejector.log
