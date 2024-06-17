# Viator sloth sli plugins

## Introduction

A collection of SLO plugins used by sloth for histogram type metrics

For integration tests, the artifacts have to be created

# Sloth plugin specifics

Each sloth SLI plugin has to be in a single file and can only use go standard modules.
See https://sloth.dev/usage/plugins/#common-plugins

Sloth SLI plugins are not build but the source code is being loaded from gitlab from the `/plugins` folder
(or packaged with the sloth slo k8 controller or loaded another way)
They are named plugin.go.

We have some common metrics code, which would have to be copied around and tested.
Normally that code would go into a go package but that is not possible with sloth plugins.

To be compatible with the normal sloth plugin loading from git, the sloth compatible
plugin files need to be in a specific folder. By default, this is the `/plugins` folder.



## `./dev-plugins` folder

This is folder is not a classic sloth plugin setup, but has been added by us to allow normal go development
and unit testing without having to duplicate code.
As part of the final build-process, the actual development files are written and maintained in `./dev-plugins`.
And then as part of the build they are being processed to be Sloth plugin compatible.
This then can be further tested with integration tests.

Later those files are being checked into the repo under `./plugins` as part of the "release" process.

This allows the default Sloth plugin git approach to use them

The development branch in git must not have any files added under `./plugins` that
need processing or have been generated.
If a plugin does not require any processing it could go under `./plugins` but either should work.


The templating is hacky, but works the following way:
Copy the relevant code over to plugins folder
Clean up imports, to makes sure the imports are correct (removing the project package ones and adding any the common code has)
Insert the common code into the file and puts it

imports (only standard libs or common code) need to be in the format of
```
import (
	"bytes"
	"context"
	"fmt"
)
```


# workflow 

Until more time is spent on this, the work and "release" process consists of these awkward steps:

1. Create new development branch off `develop`
1. Add/update plugins under dev-plugins
1. Commit all changes (make sure there aren't any local changes) and then
    1. Locally run `./scripts/build/create-plugins.sh` and then check the processed version works well with:
    1. `make check test integration-test`
    1. if everything passed check what changed and revert to the last commit:
    1. `git reset --hard HEAD`
    1. `git clean -f -x plugins/`
1. Push the plugin changes and create an MR and merge to `develop` (github will will run `check`, `unit-test`)
1. Locally checkout `develop`
1. Create a new branch off `develop` e.g. `prep-v1.1.0` (this will be merged to `main`)
1. Run the preparation steps and push the changes
    1. `./scripts/build/create-plugins.sh` and
    1. `git rm -r dev-plugins  && git add plugins/`
    1. `git commit -m "prepare next release"` && `git push`
    1. github will run `check`, `unit-test`  as well as `integration-test`
1. Create an MR to `main`
1. Once merged, tag the release and use the new tag as the current release for sloth , after testing in ninjas change sloth to use the new version









