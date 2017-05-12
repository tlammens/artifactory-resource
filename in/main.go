package main

import (
	"fmt"
	chelper "github.com/ArthurHlt/go-concourse-helper"
	"github.com/jfrogdev/jfrog-cli-go/artifactory/commands"
	artutils "github.com/jfrogdev/jfrog-cli-go/artifactory/utils"
	"github.com/jfrogdev/jfrog-cli-go/utils/config"
	"github.com/orange-cloudfoundry/artifactory-resource/model"
	"github.com/orange-cloudfoundry/artifactory-resource/utils"
	"time"
)

type In struct {
	cmd        *chelper.InCommand
	source     model.Source
	params     model.InParams
	artdetails *config.ArtifactoryDetails
	spec       *artutils.SpecFiles
}

func main() {
	in := &In{
		cmd: chelper.NewInCommand(),
	}
	in.Run()
}
func (c *In) Run() {
	fmt.Println("p")
	cmd := c.cmd
	msg := c.cmd.Messager()
	err := cmd.Source(&c.source)
	msg.FatalIf("Error when parsing source from concourse", err)
	utils.OverrideLoggerArtifactory(c.source.LogLevel)
	err = cmd.Params(&c.params)
	msg.FatalIf("Error when parsing params from concourse", err)
	c.defaultingParams()

	c.artdetails, err = utils.RetrieveArtDetails(c.source)
	if err != nil {
		msg.Fatal(err.Error())
	}
	dest := utils.AddTrailingSlashIfNeeded(c.cmd.DestinationFolder())
	if c.params.Filename != "" {
		dest += c.params.Filename
	}
	filePath := c.cmd.Version().BuildNumber
	c.spec = artutils.CreateSpec(filePath, dest, c.source.Props, false, false, false)
	msg.Log("[blue]Downloading[reset] file '[blue]%s[reset]'...", filePath)
	startDl := time.Now()
	err = c.Download()
	msg.FatalIf("Error when downloading", err)
	elapsed := time.Since(startDl)
	msg.Log("[blue]Finished downloading[reset] file '[blue]%s[reset]'.", filePath)
	metadata := []chelper.Metadata{
		{
			Name:  "downloaded_file",
			Value: filePath,
		},
		{
			Name:  "download_time",
			Value: elapsed.String(),
		},
	}
	cmd.Send(metadata)
}

func (c *In) defaultingParams() {
	if c.params.Threads <= 0 {
		c.params.Threads = 3
	}
	if c.params.SplitCount <= 0 {
		c.params.SplitCount = 3
	}
	if c.params.MinSplit <= 0 {
		c.params.MinSplit = 5120
	}
}

func (c In) Download() error {
	return commands.Download(
		c.spec,
		&commands.DownloadFlags{
			ArtDetails:   c.artdetails,
			Threads:      c.params.Threads,
			SplitCount:   c.params.SplitCount,
			MinSplitSize: int64(c.params.MinSplit),
		},
	)
}
