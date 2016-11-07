vault-crowdsource
=================
Vault Crowdsource is a small go web server that connects to a Vault server to
allow an audience to participate in interacting with Vault.

The default port is 6789, but this is configurable via the `-listen` flag:

```shell
$ vault-crowdsource -listen=:8080"
```

This tool obeys the standard Vault environment variables like `VAULT_ADDR` and
`VAULT_TOKEN`, but also requires a `VAULT_ENDPOINT` to be specified. This is the
address to the publicly accessible page where tokens can be created.

You must define a policy in vault named "crowdsource" with the permissions you
desire the created tokens to have.

Then visit http://localhost:8080/ in your browser.
