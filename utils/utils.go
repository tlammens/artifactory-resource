package utils

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	artlog "github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/orange-cloudfoundry/artifactory-resource/model"
)

const (
	ART_SECURITY_FOLDER = "security/"
)

func CheckReqParamsWithPattern(source model.Source) error {
	if source.Pattern == "" {
		return errors.New("You must provide a pattern for your file (e.g.: 'local_generic/myfile.txt','local_generic/my*.txt'.")
	}
	return CheckReqParams(source)
}

func CheckReqParams(source model.Source) error {
	if source.Url == "" {
		return errors.New("You must pass an url to artifactory.")
	}
	if source.User == "" && source.ApiKey == "" {
		return errors.New("You must pass user/password pair or apiKey to authnticate over artifactory.")
	}
	return nil
}

func RetrieveArtDetails(source model.Source) (*config.ServerDetails, error) {
	err := createCert(source.CACert)
	if err != nil {
		return nil, err
	}
	sshKeyPath, err := createSshKeyPath(source.SshKey)
	return &config.ServerDetails{
		ArtifactoryUrl: AddTrailingSlashIfNeeded(source.Url),
		Url:            AddTrailingSlashIfNeeded(source.Url),
		User:           source.User,
		Password:       source.Password,
		SshKeyPath:     sshKeyPath,
	}, nil

}

func AddTrailingSlashIfNeeded(path string) string {
	if path != "" && !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return path
}

func RemoveStartingSlashIfNeeded(path string) string {
	if path != "" && strings.HasPrefix(path, "/") {
		path = strings.TrimPrefix(path, "/")
	}
	return path
}

func createCert(caCert string) error {
	if caCert == "" {
		return nil
	}
	confPath, err := coreutils.GetJfrogHomeDir()
	if err != nil {
		return err
	}
	securityPath := confPath + ART_SECURITY_FOLDER
	os.MkdirAll(securityPath, os.ModePerm)
	return ioutil.WriteFile(securityPath+"cert.pem", []byte(caCert), 0644)
}

func createSshKeyPath(sshKey string) (string, error) {
	if sshKey == "" {
		return "", nil
	}
	file, err := ioutil.TempFile(os.TempDir(), "ssh-key")
	if err != nil {
		return "", err
	}
	_, err = file.WriteString(sshKey)
	if err != nil {
		return "", err
	}
	return filepath.Abs(filepath.Dir(file.Name()))
}

func OverrideLoggerArtifactory(logLevel string) {
	lvl := artlog.INFO
	if strings.ToUpper(logLevel) == "ERROR" {
		lvl = artlog.ERROR
	} else if strings.ToUpper(logLevel) == "DEBUG" {
		lvl = artlog.DEBUG
	}
	logger := artlog.NewLogger(lvl, os.Stderr)
	artlog.SetLogger(logger)
}
