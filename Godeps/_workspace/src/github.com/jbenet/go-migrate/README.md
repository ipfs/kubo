# go-migrate

This is a very simple migration framework. See "Migrations" in https://github.com/jbenet/random-ideas/issues/33

This package includes:

- `migrate` package -- lib to write migration programs

## The model

The idea here is that we have some thing -- usually a directory -- that needs to be migrated between different representation versions. This may be because there has been an upgrade.
