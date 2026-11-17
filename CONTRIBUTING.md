# Contributing

## Adding a plugin

A plugin folder holds four kinds of file: `plugin.yaml` (written by hand), `openapi.json` (extracted), `zz_generated.go` (generated) and acceptance tests. Nothing in `internal/core` should need to change; if it does, open an issue first.

1. Create the folder:

   ```sh
   make plugin NAME=bgp PACKAGE=netbox-bgp STATUS_KEY=netbox_bgp API_PREFIX=bgp
   ```

   `STATUS_KEY` is the plugin's key under `plugins` in NetBox's `/api/status/`; `API_PREFIX` is the segment in `/api/plugins/<prefix>/`.

2. Add the plugin's pip package to the test profiles in `testenv/profiles/` and its module name to `testenv/plugins.py`, then start an environment:

   ```sh
   make testenv-up PROFILE=netbox-4.7
   ```

3. Extract the plugin's part of the schema:

   ```sh
   . testenv/.env.netbox-4.7
   go run ./tools/extract -prefix bgp -status-key netbox_bgp -out internal/plugins/bgp/openapi.json
   ```

4. List the models in `plugin.yaml`, one entry per endpoint, with the resource suffix in `name`. Leave out endpoints that expose core NetBox objects. Set `compatibility` to the NetBox and plugin versions you test.

5. Generate and register the folder:

   ```sh
   go generate ./internal/plugins/bgp/
   ```

   Add the package to `internal/plugins/registry.go` and the folder to `.github/CODEOWNERS` with your handle.

6. Write `acc_test.go` covering every model: create, import, a change made through the API on each attribute kind (`acctest.DriftSteps`), and an update. See `internal/plugins/dns/acc_test.go`. Run it:

   ```sh
   make testacc PROFILE=netbox-4.7 PKGS=./internal/plugins/bgp/
   ```

7. Regenerate the docs (`make generate`) and open a pull request.

### Overrides

When the generated description of a field is wrong for Terraform, correct it in `plugin.yaml`:

```yaml
models:
  - endpoint: zones
    name: zone
    overrides:
      soa_serial: { volatile: true } # the plugin rewrites it when the zone changes
      some_field: { skip: true }     # not exposed
      ports: { set: true }           # order carries no meaning
```

Available keys: `skip`, `name`, `kind`, `set`, `sensitive`, `read_only`, `volatile`, `description`. A test failing with "Provider produced inconsistent result after apply" on a field you did not set usually means that field is `volatile`.

## Supporting a new NetBox or plugin version

Add a profile in `testenv/profiles/`, run the acceptance tests against it, and add the passing combination to `compatibility` in `plugin.yaml`. If the schema changed, extract it again from the newest version and regenerate.

## Checks

Pull requests run unit tests, `golangci-lint`, a check that generated files are current, and the acceptance tests of the changed plugin folders on every test profile with OpenTofu and Terraform. A change outside `internal/plugins/<name>/` runs every folder.

## Decisions

Design changes start with an ADR in `design/adr/`, copied from `0000-template.md`.
