# SyncSet/SelectorSyncSet Generator

Creates a SyncSet or SelectorSyncSet resource by recursively parsing all manifests of a given path.

```
Usage:
  ss view [flags]

Flags:
  -c, --cluster-name string   The cluster name used to create a SyncSet
  -h, --help                  help for view
  -p, --path string           The path of the manifest files to use (default ".")
  -s, --selector string       The selector key/value pair used to create a SelectorSyncSet
```
