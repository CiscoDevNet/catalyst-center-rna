# RNA for Cisco DNA Center

RNA (Rapid Network Assesment) is a tool is designed to collect data from Cisco DNA Center for the purpose of conducting Health Checks, which is performed by Cisco Services.

This client is based on <https://github.com/aci-vetr/collector> so it is expected to see `APIC`, `aci` references on the code.

## Purpose

RNA performs data collection from Cisco DNA Center to perform Health Checks. It can be run from any computer with access to Cisco DNA Center.

Once the collection is complete, RNA will create a `dnac_rna_hc_collection.zip` file.

This file should be provided to the Cisco Customer Success Specialist for further analysis.

In addition, RNA also creates a log file that can be reviewed and/or provided to Cisco to troubleshoot any issues with the collection process. Note, that this file will only be available in a failure scenario; upon successful collection this file is bundled into the `dnac_rna_hc_collection.zip` file along with collection data.

## How it works

RNA collects data from a various APIs on Cisco DNA Center. The data collected through these queries is stored in a zip file to be shared with Cisco. RNA does not currently interact with end devices and instead retrieves all data from the DNAC via its APIs.

The following file can be referenced to see the API queries performed by RNA:

<https://wwwin-github.cisco.com/netascode/dnac-collector/blob/master/pkg/req/reqs.yaml>

## Safety/Security

Please note the following points regarding the data collection performed by RNA:

- All of the queries performed by RNA are also performed by Cisco DNA Center GUI, so there is no more risk than clicking through the GUI.
- Queries to Cisco DNA Center are batched and throttled to ensure reduced load on Cisco DNA Center.
- Cisco DNA Center has internal safeguards to protect against excess API usage.
- API interaction in Cisco DNA Center has no impact on data forwarding behavior.
- RNA is open source and can be compiled manually with the Go compiler.

Credentials are only used at the point of collection and are not stored in any way.

All data provided to Cisco will be maintained under Cisco's [data retention policy](https://www.cisco.com/c/en/us/about/trust-center/global-privacy-policy.html).

## Usage

All command line parameters are optional; RNA will prompt for any missing information. Use the `--help` option to see this output from the CLI.

**Note** that only `dnac`, `username`, and `password` are typically required. The remainder of the options exist to work around uncommon connectivity challenges, e.g. a long RTT or slow response from the DNAC.

```bash
$ ./dnac_rna --help
RNA - Rapid Network Assesment for Cisco DNA Center.
version 1.0.0
Usage: dnac_rna [--address ADDRESS] [--username USERNAME] [--password PASSWORD] [--output OUTPUT] [--request-retry-count REQUEST-RETRY-COUNT] [--retry-delay RETRY-DELAY] [--batch-size BATCH-SIZE]

Options:
  --address ADDRESS, -a ADDRESS
                         Cisco DNA Center hostname or IP address
  --username USERNAME, -u USERNAME
                         Cisco DNA Center username
  --password PASSWORD, -p PASSWORD
                         Cisco DNA Center password
  --output OUTPUT, -o OUTPUT
                         Output file [default: dnac_rna_hc_collection.zip]
  --request-retry-count REQUEST-RETRY-COUNT
                         Times to retry a failed request [default: 3]
  --retry-delay RETRY-DELAY
                         Seconds to wait before retry [default: 10]
  --batch-size BATCH-SIZE
                         Max request to send in parallel [default: 10]
  --help, -h             display this help and exit
  --version              display version and exit
```

### Running Binary file

Select the latest binary corresponding to the platform you are using from the [release tab](https://wwwin-github.cisco.com/netascode/dnac-collector/releases) and unzip the file.

On Windows you can double click the binary file.

In Linux and MacOS use the command below from the same directory where the binary is located.

```bash
./dnac_rna
```

In MacOS, you may receive a warning saying: _cannot be opened because the developer cannot be verified_. To run the collector allow the execution from `preferences > privacy & security`, scroll to the section `security` and you will see a message saying _"collector" was blocked ..._, click `Open Anyway` and run the collector again. A popup will appear asking if you want to run it, click yes. After this, you will be able to run the collector script.

### Running code directly from source

If you are familiar with `Go`, you have the option to run the program directly from the source code. To do this, execute the code from the root of the repository. This method is useful for auditing the program's functionality and is particularly helpful for security audits. However, it is important to note that the majority of users should not run the program in this way.

```bash
go mod download
go run ./*.go
```

### Environment Variables

You can use **environment variables** to run RNA.

Create a `.env` file on the same directoy where RNA is located and add your Cisco DNA Center credentials and IP/host using variables below.

- `DNAC_IP=<DNAC-IP>`
- `DNAC_USER=<DNAC-USER>`
- `PASSWORD=<DNAC-PASSWORD>`

This is an example of a `.env` file.

```bash
$ cat .env
DNAC_IP=https://sandboxdnac.cisco.com
DNAC_USER=devnetuser
PASSWORD=Cisco123!
```
