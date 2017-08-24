# Artifactory Resource

A concourse resource for push and download files from/to artifactory with semver.

## Source Configuration

* `url`: *Required.* Url which target your artifactory.

* `user`: *Optional.* Artifactory username.

* `password`: *Optional.* Artifactory password.

* `ssh_key`: *Optional.* Artifactory ssh key.

* `pattern`: *Required for in.* Pattern to use to find file (you can use glob format or regex if `regexp` set to `true`).

* `props`: *Optional.* List of properties in the form of "key1=value1;key2=value2,..." Only artifacts with these properties will be downloaded.

* `recursive`: *Default: true* Set to false if you do not wish to include the download of artifacts inside sub-folders in Artifactory.

* `flat`: *Default: false* Set to true if you do not wish to have the Artifactory repository path structure created locally for your downloaded files.

* `regexp`: *Default: false* If true, `pattern` will interpret as a regular expression.

* `version`: *Optional.* If set resource will filter files found with matching semver set (e.g.: `0.5.x`)

* `log_level`: *Default: `INFO`* Set the verbosity of logs, other values are: `ERROR`, `WARN`, `DEBUG`.

* `ca_cert`: *Optional.* Pass a certificate to access to your artifactory.



## Behavior

### `check`: Check for new files.

Find all files matching the pattern and filter by their version if `version` is set.


### `in`: Download a file from Artifactory


#### Parameters

* `filename`: *Optional.* If set filename for the downloaded file will be overwritten by this name.

* `not_flat`: *Optional.* If true artifacts are downloaded to the target path in the file system while maintaining their hierarchy in the source repository.

* `min_split`: *Default: 5120* The minimum size permitted for splitting. Files larger than the specified number will be split into equally sized `split_count` segments. 
Any files smaller than the specified number will be downloaded in a single thread. If set to -1, files are not split.

* `split_count`: *Default: 3* The number of segments into which each file should be split for download (provided the artifact is over --min-split in size). To download each file in a single thread, set to 0.

* `props_filename`: *Optional.* Path to file where Artifactory properties of downloaded file will be stored. File will contain whole REST API response and properties values can be extracted with other tools like jq. If parameter is empty - no request to Artifactory will be made.

### `out`: Upload a file to artifactory.

#### Parameters

* `target`: *Required.* An artifactory repository in the format of `[repository_name]/[repository_path]`.

* `source`: *Required.* Pattern which target a set of files or a file (can use glob format).

* `threads`: *Default: 3* The number of parallel threads that should be used to download where each thread downloads a single artifact at a time.

* `explode_archive`: *Default: false* If true, the command will extract an archive containing multiple artifacts after it is deployed to Artifactory, while maintaining the archive's file structure.

* `props`: *Optional.* List of properties in the form of "key1=value1;key2=value2,...". Those properties will be added to uploaded file. If both `props` and `props_from_file` are set values will be merged.

* `props_from_file`: *Optional.* Path to file which will contain list of properties. List should be in the form of "key1=value1;key2=value2,...". Those properties will be added to uploaded file. If both `props` and `props_from_file` are set values will be merged.

## Example

``` yaml
resource_types:
- name: artifactory
  type: docker-image
  source:
    repository: orangeopensource/artifactory-resource
resources:
- name: artifactory-resource
  type: artifactory
  source:
    url: https://my.artifactory.com
    user: myuser
    password: mypassword
    pattern: "bosh_release/**/credhub-*.tgz"
    version: "0.5.x"

jobs:
- name: build-rootfs
  plan:
  - get: artifactory-resource
    filename: credhub.tgz
  - put: artifactory-resource
    params:
      target: bosh_release/credhub/
      source: credhub.tgz
```
