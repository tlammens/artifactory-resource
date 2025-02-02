package utils

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/utils/ioutils"
	npmutils "github.com/jfrog/jfrog-cli-core/v2/utils/npm"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/buildinfo"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	serviceutils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/http/httpclient"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/utils/version"
	"github.com/pkg/errors"
)

const minSupportedArtifactoryVersionForNpmCmds = "5.5.2"

func GetArtifactoryNpmRepoDetails(repo string, authArtDetails *auth.ServiceDetails) (npmAuth, registry string, err error) {
	npmAuth, err = getNpmAuth(authArtDetails)
	if err != nil {
		return "", "", err
	}

	if err = utils.CheckIfRepoExists(repo, *authArtDetails); err != nil {
		return "", "", err
	}

	registry = getNpmRepositoryUrl(repo, (*authArtDetails).GetUrl())
	return
}

func getNpmAuth(authArtDetails *auth.ServiceDetails) (npmAuth string, err error) {
	// Check Artifactory version
	err = validateArtifactoryVersionForNpmCmds(authArtDetails)
	if err != nil {
		return
	}

	// Get npm token from Artifactory
	if (*authArtDetails).GetAccessToken() == "" {
		return getNpmAuthUsingBasicAuth(authArtDetails)
	}
	return getNpmAuthUsingAccessToken(authArtDetails)
}

func validateArtifactoryVersionForNpmCmds(artDetails *auth.ServiceDetails) error {
	// Get Artifactory version.
	versionStr, err := (*artDetails).GetVersion()
	if err != nil {
		return err
	}

	// Validate version.
	rtVersion := version.NewVersion(versionStr)
	if !rtVersion.AtLeast(minSupportedArtifactoryVersionForNpmCmds) {
		return errorutils.CheckError(errors.New("this operation requires Artifactory version " + minSupportedArtifactoryVersionForNpmCmds + " or higher"))
	}

	return nil
}

func getNpmAuthUsingAccessToken(artDetails *auth.ServiceDetails) (npmAuth string, err error) {
	npmAuthString := "_auth = %s\nalways-auth = true"
	// Build npm token, consists of <username:password> encoded.
	// Use Artifactory's access-token as username and password to create npm token.
	username, err := auth.ExtractUsernameFromAccessToken((*artDetails).GetAccessToken())
	if err != nil {
		return
	}

	encodedNpmToken := base64.StdEncoding.EncodeToString([]byte(username + ":" + (*artDetails).GetAccessToken()))
	npmAuth = fmt.Sprintf(npmAuthString, encodedNpmToken)

	return
}

func getNpmAuthUsingBasicAuth(artDetails *auth.ServiceDetails) (npmAuth string, err error) {
	authApiUrl := (*artDetails).GetUrl() + "api/npm/auth"
	log.Debug("Sending npm auth request")

	// Get npm token from Artifactory.
	client, err := httpclient.ClientBuilder().SetRetries(3).Build()
	if err != nil {
		return "", err
	}
	resp, body, _, err := client.SendGet(authApiUrl, true, (*artDetails).CreateHttpClientDetails(), "")
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", errorutils.CheckError(errors.New("Artifactory response: " + resp.Status + "\n" + clientutils.IndentJson(body)))
	}

	return string(body), nil
}

func getNpmRepositoryUrl(repo, url string) string {
	if !strings.HasSuffix(url, "/") {
		url += "/"
	}
	url += "api/npm/" + repo
	return url
}

func PrepareBuildInfo(workingDirectory string, buildConfiguration *utils.BuildConfiguration, npmVersion *version.Version) (collectBuildInfo bool, packageInfo *npmutils.PackageInfo, err error) {
	if len(buildConfiguration.BuildName) > 0 && len(buildConfiguration.BuildNumber) > 0 {
		collectBuildInfo = true
		if err = utils.SaveBuildGeneralDetails(buildConfiguration.BuildName, buildConfiguration.BuildNumber, buildConfiguration.Project); err != nil {
			return false, nil, err
		}

		if packageInfo, err = npmutils.ReadPackageInfoFromPackageJson(workingDirectory, npmVersion); err != nil {
			return false, nil, err
		}
	}

	return
}

// Get dependency's checksum and type.
func GetDependencyInfo(name, ver string, previousBuildDependencies map[string]*buildinfo.Dependency,
	servicesManager artifactory.ArtifactoryServicesManager, threadId int) (checksum *buildinfo.Checksum, fileType string, err error) {
	id := name + ":" + ver
	if dep, ok := previousBuildDependencies[id]; ok {
		// Get checksum from previous build.
		checksum = dep.Checksum
		fileType = dep.Type
		return
	}

	// Get info from Artifactory.
	log.Debug(clientutils.GetLogMsgPrefix(threadId, false), "Fetching checksums for", name, ":", ver)
	var stream io.ReadCloser
	stream, err = servicesManager.Aql(serviceutils.CreateAqlQueryForNpm(name, ver))
	if err != nil {
		return
	}
	defer stream.Close()
	var result []byte
	result, err = ioutil.ReadAll(stream)
	if err != nil {
		return
	}
	parsedResult := new(aqlResult)
	if err = json.Unmarshal(result, parsedResult); err != nil {
		return nil, "", errorutils.CheckError(err)
	}
	if len(parsedResult.Results) == 0 {
		log.Debug(clientutils.GetLogMsgPrefix(threadId, false), name, ":", ver, "could not be found in Artifactory.")
		return
	}
	if i := strings.LastIndex(parsedResult.Results[0].Name, "."); i != -1 {
		fileType = parsedResult.Results[0].Name[i+1:]
	}
	log.Debug(clientutils.GetLogMsgPrefix(threadId, false), "Found", parsedResult.Results[0].Name,
		"sha1:", parsedResult.Results[0].Actual_sha1,
		"md5", parsedResult.Results[0].Actual_md5)

	checksum = &buildinfo.Checksum{Sha1: parsedResult.Results[0].Actual_sha1, Md5: parsedResult.Results[0].Actual_md5}
	return
}

type aqlResult struct {
	Results []*results `json:"results,omitempty"`
}

type results struct {
	Name        string `json:"name,omitempty"`
	Actual_md5  string `json:"actual_md5,omitempty"`
	Actual_sha1 string `json:"actual_sha1,omitempty"`
}

func GetDependenciesFromLatestBuild(servicesManager artifactory.ArtifactoryServicesManager, buildName string) (map[string]*buildinfo.Dependency, error) {
	buildDependencies := make(map[string]*buildinfo.Dependency)
	previousBuild, found, err := servicesManager.GetBuildInfo(services.BuildInfoParams{BuildName: buildName, BuildNumber: "LATEST"})
	if err != nil || !found {
		return buildDependencies, err
	}
	for _, module := range previousBuild.BuildInfo.Modules {
		for _, dependency := range module.Dependencies {
			buildDependencies[dependency.Id] = &buildinfo.Dependency{Id: dependency.Id, Type: dependency.Type,
				Checksum: &buildinfo.Checksum{Md5: dependency.Md5, Sha1: dependency.Sha1}}
		}
	}
	return buildDependencies, nil
}

func ExtractNpmOptionsFromArgs(args []string) (threads int, detailedSummary, xrayScan bool, scanOutputFormat audit.OutputFormat, cleanArgs []string, buildConfig *utils.BuildConfiguration, err error) {
	threads = 3
	// Extract threads information from the args.
	flagIndex, valueIndex, numOfThreads, err := coreutils.FindFlag("--threads", args)
	if err != nil {
		return
	}
	coreutils.RemoveFlagFromCommand(&args, flagIndex, valueIndex)
	if numOfThreads != "" {
		threads, err = strconv.Atoi(numOfThreads)
		if err != nil {
			err = errorutils.CheckError(err)
			return
		}
	}

	flagIndex, detailedSummary, err = coreutils.FindBooleanFlag("--detailed-summary", args)
	if err != nil {
		return
	}
	// Since boolean flag might appear as --flag or --flag=value, the value index is the same as the flag index.
	coreutils.RemoveFlagFromCommand(&args, flagIndex, flagIndex)

	flagIndex, xrayScan, err = coreutils.FindBooleanFlag("--scan", args)
	if err != nil {
		return
	}
	// Since boolean flag might appear as --flag or --flag=value, the value index is the same as the flag index.
	coreutils.RemoveFlagFromCommand(&args, flagIndex, flagIndex)

	flagIndex, valueIndex, format, err := coreutils.FindFlag("--format", args)
	if err != nil {
		return
	}
	scanOutputFormat, err = GetXrayOutputFormat(format)
	if err != nil {
		return
	}
	coreutils.RemoveFlagFromCommand(&args, flagIndex, valueIndex)

	cleanArgs, buildConfig, err = utils.ExtractBuildDetailsFromArgs(args)
	return
}

func SaveDependenciesData(dependencies []buildinfo.Dependency, buildConfiguration *utils.BuildConfiguration) error {
	populateFunc := func(partial *buildinfo.Partial) {
		partial.Dependencies = dependencies
		partial.ModuleId = buildConfiguration.Module
		partial.ModuleType = buildinfo.Npm
	}

	return utils.SavePartialBuildInfo(buildConfiguration.BuildName, buildConfiguration.BuildNumber, buildConfiguration.Project, populateFunc)
}

func PrintMissingDependencies(missingDependencies []buildinfo.Dependency) {
	if len(missingDependencies) == 0 {
		return
	}

	var missingDependenciesText []string
	for _, dependency := range missingDependencies {
		missingDependenciesText = append(missingDependenciesText, dependency.Id)
	}
	log.Warn(strings.Join(missingDependenciesText, "\n"))
	log.Warn("The npm dependencies above could not be found in Artifactory and therefore are not included in the build-info.\n" +
		"Make sure the dependencies are available in Artifactory for this build.\n" +
		"Deleting the local cache will force populating Artifactory with these dependencies.")
}

// BackupFile creates a backup of the file in filePath. The backup will be found at backupPath.
// The returned restore function can be called to restore the file's state - the file in filePath will be replaced by the backup in backupPath.
// If there is no file at filePath, a backup file won't be created, and the restore function will delete the file at filePath.
func BackupFile(filePath, backupPath string) (restore func() error, err error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return createRestoreFileFunc(filePath, backupPath), nil
		}
		return nil, errorutils.CheckError(err)
	}

	fileMode := fileInfo.Mode()
	if err = ioutils.CopyFile(filePath, backupPath, fileMode); err != nil {
		return nil, err
	}
	log.Debug("The file", filePath, "was backed up successfully to", backupPath)
	return createRestoreFileFunc(filePath, backupPath), nil
}

// createRestoreFileFunc creates a function for restoring a file from its backup.
// The returned function replaces the file in filePath with the backup in backupPath.
// If there is no file at backupPath (which means there was no file at filePath when BackupFile() was called), then the function deletes the file at filePath.
func createRestoreFileFunc(filePath, backupPath string) func() error {
	return func() error {
		if _, err := os.Stat(backupPath); err != nil {
			if os.IsNotExist(err) {
				err = os.Remove(filePath)
				return errorutils.CheckError(err)
			}
			return errorutils.CheckError(errors.New(createRestoreErrorPrefix(filePath, backupPath) + err.Error()))
		}

		if err := fileutils.MoveFile(backupPath, filePath); err != nil {
			return errorutils.CheckError(err)
		}
		log.Debug("Restored the file", filePath, "successfully")

		return nil
	}
}

func createRestoreErrorPrefix(filePath, backupPath string) string {
	return fmt.Sprintf("An error occurred while restoring the file: %s\n"+
		"To restore the file manually: delete %s and rename the backup file at %s (if exists) to '%s'.\n"+
		"Failure cause: ",
		filePath, filePath, backupPath, filePath)
}
