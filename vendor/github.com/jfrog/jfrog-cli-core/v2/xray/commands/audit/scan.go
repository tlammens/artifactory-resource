package audit

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/jfrog/gofrog/io"
	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands"
	xrutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/artifactory/services/fspatterns"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

type FileContext func(string) parallel.TaskFunc
type indexFileHandlerFunc func(file string)
type OutputFormat string

const (
	// OutputFormat values
	Table OutputFormat = "table"
	Json  OutputFormat = "json"

	indexingCommand          = "graph"
	fileNotSupportedExitCode = 3
)

type ScanCommand struct {
	serverDetails *config.ServerDetails
	spec          *spec.SpecFiles
	threads       int
	// The location of the downloaded Xray indexer binary on the local file system.
	indexerPath            string
	outputFormat           OutputFormat
	projectKey             string
	watches                []string
	includeVulnerabilities bool
	includeLincenses       bool
	scanPassed             bool
}

func (scanCmd *ScanCommand) SetThreads(threads int) *ScanCommand {
	scanCmd.threads = threads
	return scanCmd
}

func (scanCmd *ScanCommand) SetOutputFormat(format OutputFormat) *ScanCommand {
	scanCmd.outputFormat = format
	return scanCmd
}

func (scanCmd *ScanCommand) SetServerDetails(server *config.ServerDetails) *ScanCommand {
	scanCmd.serverDetails = server
	return scanCmd
}

func (scanCmd *ScanCommand) SetSpec(spec *spec.SpecFiles) *ScanCommand {
	scanCmd.spec = spec
	return scanCmd
}

func (scanCmd *ScanCommand) SetProject(project string) *ScanCommand {
	scanCmd.projectKey = project
	return scanCmd
}

func (scanCmd *ScanCommand) SetWatches(watches []string) *ScanCommand {
	scanCmd.watches = watches
	return scanCmd
}

func (scanCmd *ScanCommand) SetIncludeVulnerabilities(include bool) *ScanCommand {
	scanCmd.includeVulnerabilities = include
	return scanCmd
}

func (scanCmd *ScanCommand) SetIncludeLincenses(include bool) *ScanCommand {
	scanCmd.includeLincenses = include
	return scanCmd
}

func (scanCmd *ScanCommand) ServerDetails() (*config.ServerDetails, error) {
	return scanCmd.serverDetails, nil
}

func (scanCmd *ScanCommand) IsScanPassed() bool {
	return scanCmd.scanPassed
}

func (scanCmd *ScanCommand) indexFile(filePath string) (*services.GraphNode, error) {
	var indexerResults services.GraphNode
	indexCmd := &coreutils.GeneralExecCmd{
		ExecPath: scanCmd.indexerPath,
		Command:  []string{indexingCommand, filePath},
	}
	output, err := io.RunCmdOutput(indexCmd)
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			if e.ExitCode() == fileNotSupportedExitCode {
				log.Debug(fmt.Sprintf("File %s is not supported by Xray indexr app.", filePath))
			}
			return &indexerResults, nil
		}
		return nil, errorutils.CheckError(err)
	}
	err = json.Unmarshal([]byte(output), &indexerResults)
	return &indexerResults, errorutils.CheckError(err)
}

func (scanCmd *ScanCommand) getXrScanGraphResults(graph *services.GraphNode, file *spec.File) (*services.ScanResponse, error) {
	xrayManager, err := commands.CreateXrayServiceManager(scanCmd.serverDetails)
	if err != nil {
		return nil, err
	}
	params := services.NewXrayGraphScanParams()
	params.RepoPath = getXrayRepoPathFromTarget(file.Target)
	params.Watches = scanCmd.watches
	params.Graph = graph
	scanId, err := xrayManager.ScanGraph(params)
	if err != nil {
		return nil, err
	}
	scanResults, err := xrayManager.GetScanGraphResults(scanId, scanCmd.includeVulnerabilities, scanCmd.includeLincenses)
	if err != nil {
		return nil, err
	}
	return scanResults, nil
}

func (scanCmd *ScanCommand) Run() (err error) {
	// First download Xray Indexer if needed
	xrayManager, err := commands.CreateXrayServiceManager(scanCmd.serverDetails)
	if err != nil {
		return err
	}
	scanCmd.indexerPath, err = xrutils.DownloadIndexerIfNeeded(xrayManager)
	if err != nil {
		return err
	}
	threads := 1
	if scanCmd.threads > 1 {
		threads = scanCmd.threads
	}
	resultsArr := make([][]*services.ScanResponse, threads)
	fileProducerConsumer := parallel.NewRunner(scanCmd.threads, 20000, false)
	fileProducerErrorsQueue := clientutils.NewErrorsQueue(1)
	indexedFileProducerConsumer := parallel.NewRunner(scanCmd.threads, 20000, false)
	indexedFileProducerErrorsQueue := clientutils.NewErrorsQueue(1)
	// Start walking on the filesystem to "produce" files that match the given pattern
	// while the consumer uses the indexer to index those files.
	scanCmd.prepareScanTasks(fileProducerConsumer, indexedFileProducerConsumer, resultsArr, fileProducerErrorsQueue, indexedFileProducerErrorsQueue)
	scanCmd.scanPassed, err = scanCmd.performScanTasks(fileProducerConsumer, indexedFileProducerConsumer, resultsArr)
	if err != nil {
		return err
	}
	err = fileProducerErrorsQueue.GetError()
	if err != nil {
		return err
	}
	return indexedFileProducerErrorsQueue.GetError()
}

func NewScanCommand() *ScanCommand {
	return &ScanCommand{}
}

func (scanCmd *ScanCommand) CommandName() string {
	return "xr_scan"
}

func (scanCmd *ScanCommand) prepareScanTasks(fileProducer, indexedFileProducer parallel.Runner, resultsArr [][]*services.ScanResponse, fileErrorsQueue, indexedFileErrorsQueue *clientutils.ErrorsQueue) {
	go func() {
		defer fileProducer.Done()
		// Iterate over file-spec groups and produce indexing tasks.
		// When encountering an error, log and move to next group.
		for _, fileGroup := range scanCmd.spec.Files {
			artifactHandlerFunc := scanCmd.createIndexerHandlerFunc(&fileGroup, indexedFileProducer, resultsArr, indexedFileErrorsQueue)
			taskHandler := getAddTaskToProducerFunc(fileProducer, fileErrorsQueue, artifactHandlerFunc)

			err := collectFilesForIndexing(fileGroup, taskHandler)
			if err != nil {
				log.Error(err)
				fileErrorsQueue.AddError(err)
			}
		}
	}()
}

func (scanCmd *ScanCommand) createIndexerHandlerFunc(file *spec.File, indexedFileProducer parallel.Runner, resultsArr [][]*services.ScanResponse, errorsQueue *clientutils.ErrorsQueue) FileContext {
	return func(filePath string) parallel.TaskFunc {
		return func(threadId int) (err error) {
			logMsgPrefix := clientutils.GetLogMsgPrefix(threadId, false)
			log.Info(logMsgPrefix+"Indexing file:", filePath)
			graph, err := scanCmd.indexFile(filePath)
			if err != nil {
				return err
			}
			// In case of empty graph returned by the indexer,
			// for instance due to unsupported file format, continue without sendding a
			// graph request to Xray.
			if graph.Id == "" {
				return nil
			}
			// Add a new task to the seconde prodicer/consumer
			// which will send the indexed binary to Xray and then will store the received result.
			taskFunc := func(threadId int) (err error) {
				scanResults, err := scanCmd.getXrScanGraphResults(graph, file)
				if err != nil {
					return err
				}
				resultsArr[threadId] = append(resultsArr[threadId], scanResults)
				return
			}

			indexedFileProducer.AddTaskWithError(taskFunc, errorsQueue.AddError)
			return
		}
	}
}

func getAddTaskToProducerFunc(producer parallel.Runner, errorsQueue *clientutils.ErrorsQueue, fileHandlerFunc FileContext) indexFileHandlerFunc {
	return func(filePath string) {
		taskFunc := fileHandlerFunc(filePath)
		producer.AddTaskWithError(taskFunc, errorsQueue.AddError)
	}
}

func (scanCmd *ScanCommand) performScanTasks(fileConsumer parallel.Runner, indexedFileConsumer parallel.Runner, resultsArr [][]*services.ScanResponse) (bool, error) {
	go func() {
		// Blocking until consuming is finished.
		fileConsumer.Run()
		// After all files has been indexed, The seconde producer notifies that no more tasks will be produced.
		indexedFileConsumer.Done()
	}()
	// Blocking until consuming is finished.
	indexedFileConsumer.Run()
	// Handle results
	scanPassed := true
	flatResults := []services.ScanResponse{}
	for _, arr := range resultsArr {
		for _, res := range arr {
			flatResults = append(flatResults, *res)
			if len(res.Violations) > 0 || len(res.Vulnerabilities) > 0 {
				// A violation or vulnerability was found, the scan failed.
				scanPassed = false
			}
		}
	}
	err := xrutils.PrintScanResults(flatResults, scanCmd.outputFormat == Table, scanCmd.includeVulnerabilities, scanCmd.includeLincenses, true)
	if scanPassed && err == nil {
		log.Info("Scan completed successfully.")
	}
	return scanPassed, err
}

func collectFilesForIndexing(fileData spec.File, dataHandlerFunc indexFileHandlerFunc) error {

	fileData.Pattern = (clientutils.ReplaceTildeWithUserHome(fileData.Pattern))
	patternType := fileData.GetPatternType()
	rootPath, err := fspatterns.GetRootPath(fileData.Pattern, fileData.Target, patternType, false)
	if err != nil {
		return err
	}

	isDir, err := fileutils.IsDirExists(rootPath, false)
	if err != nil {
		return err
	}

	// If the path is a single file, index it and return
	if !isDir {
		dataHandlerFunc(rootPath)
		return nil
	}
	fileData.Pattern = clientutils.ConvertLocalPatternToRegexp(fileData.Pattern, patternType)
	return collectPatternMatchingFiles(fileData, rootPath, dataHandlerFunc)
}

func collectPatternMatchingFiles(fileData spec.File, rootPath string, dataHandlerFunc indexFileHandlerFunc) error {
	fileParams, err := fileData.ToCommonParams()
	if err != nil {
		return err
	}
	excludePathPattern := fspatterns.PrepareExcludePathPattern(fileParams)
	patternRegex, err := regexp.Compile(fileData.Pattern)
	if errorutils.CheckError(err) != nil {
		return err
	}
	recursive, err := fileData.IsRecursive(true)
	if err != nil {
		return err
	}

	paths, err := fspatterns.GetPaths(rootPath, recursive, false, false)
	if err != nil {
		return err
	}
	for _, path := range paths {
		matches, isDir, _, err := fspatterns.PrepareAndFilterPaths(path, excludePathPattern, false, false, patternRegex)
		if err != nil {
			return err
		}
		// Because paths should contains all files and directories (walks recursively) we can ignore dirs, as only files relevance for indexing.
		if isDir {
			continue
		}
		if matches != nil && len(matches) > 0 {
			dataHandlerFunc(path)
		}
	}
	return nil
}

// Xray expect a path inside a repo, but not accpet path to a file.
// Therefore, if the given target path is a path to a file,
// the path to the parent directory will be return.
// Otherwise the func will return the path itself.
func getXrayRepoPathFromTarget(target string) (repoPath string) {
	if strings.HasSuffix(target, "/") {
		return target
	}
	return target[:strings.LastIndex(target, "/")+1]
}
