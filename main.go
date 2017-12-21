package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	docker "github.com/docker/docker/client"
	flags "github.com/jessevdk/go-flags"
)

const (
	newContainerSuffix = "_newContainer"
	oldContainerSuffix = "_oldContainer"
)

type options struct {
	CopyFiles          []string `short:"f" long:"copy-files" description:"Filenames to copy to new container"`
	Image              string   `short:"i" long:"image" description:"Image name for new container"`
	RemoveOldContainer bool     `long:"rm" description:"remove old container after update"`
	Verbose            bool     `short:"v" long:"verbose" description:"Show verbose debug information"`
	PosArgs            posArgs  `positional-args:"true" required:"true"`
}

type posArgs struct {
	ContainerID string `positional-arg-name:"CONTAINER_ID"`
}

var opts options

func vPrintln(args ...interface{}) {
	if opts.Verbose {
		fmt.Println(args...)
	}
}

func vPrintf(format string, args ...interface{}) {
	if opts.Verbose {
		fmt.Printf(format, args...)
	}
}

func printErr(err error) {
	fmt.Fprintln(os.Stderr, err)
}

func encode(i interface{}) string {
	buf := bytes.Buffer{}
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(i); err != nil {
		panic(err)
	}
	return buf.String()
}
func main() { os.Exit(exec()) }

func exec() int {
	if _, err := flags.Parse(&opts); err != nil {
		if fe, ok := err.(*flags.Error); ok {
			if fe.Type == flags.ErrHelp {
				return 0
			}
		}
		printErr(err)
		return 1
	}
	cli, err := docker.NewEnvClient()
	if err != nil {
		printErr(err)
		return 1
	}
	info, err := inspect(cli, opts.PosArgs.ContainerID)
	if err != nil {
		printErr(err)
		return 1
	}
	contents := []io.ReadCloser{}
	for _, file := range opts.CopyFiles {
		r, err := copyFileFromContainer(cli, opts.PosArgs.ContainerID, file)
		if err != nil {
			printErr(err)
			return 1
		}
		contents = append(contents, r)
	}
	cccBody, err := create(cli, info)
	if err != nil {
		printErr(err)
		return 1
	}
	if err := stop(cli, opts.PosArgs.ContainerID); err != nil {
		printErr(err)
		return 1
	}
	if err := start(cli, cccBody.ID); err != nil {
		printErr(err)
		return 1
	}
	for i, r := range contents {
		if err := copyFileToContainer(cli, cccBody.ID, filepath.Dir(opts.CopyFiles[i]), r); err != nil {
			printErr(err)
			return 1
		}
	}
	if err := rename(cli, opts.PosArgs.ContainerID); err != nil {
		printErr(err)
		return 1
	}
	if opts.RemoveOldContainer {
		if err := remove(cli, opts.PosArgs.ContainerID); err != nil {
			printErr(err)
			return 1
		}
	}
	return 0
}

func inspect(cli *docker.Client, cid string) (types.ContainerJSON, error) {
	vPrintf("inspect %s\n", cid)
	cj, err := cli.ContainerInspect(context.Background(), cid)
	if err != nil {
		return types.ContainerJSON{}, err
	}
	vPrintf("inspected: %s\n", encode(cj.ContainerJSONBase))
	return cj, nil
}

func create(cli *docker.Client, info types.ContainerJSON) (container.ContainerCreateCreatedBody, error) {
	vPrintf("create new container\n")
	containerConfig := info.Config
	if opts.Image != "" {
		containerConfig.Image = opts.Image
	}
	vPrintf("ContainerConfig: %s\n", encode(containerConfig))
	hostConfig := info.ContainerJSONBase.HostConfig
	vPrintf("HostConfig: %s\n", encode(hostConfig))
	networkingConfig := &network.NetworkingConfig{EndpointsConfig: info.NetworkSettings.Networks}
	vPrintf("NetworkingConfig: %s\n", encode(networkingConfig))
	return cli.ContainerCreate(context.Background(),
		containerConfig,
		hostConfig,
		networkingConfig,
		info.ContainerJSONBase.Name+newContainerSuffix)
}

func copyFileFromContainer(cli *docker.Client, cid, fpath string) (io.ReadCloser, error) {
	vPrintf("copy file from %s\n", fpath)
	r, _, err := cli.CopyFromContainer(context.Background(), cid, fpath)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func copyFileToContainer(cli *docker.Client, cid, fpath string, r io.ReadCloser) error {
	vPrintf("copy file to %s\n", fpath)
	return cli.CopyToContainer(context.Background(), cid, filepath.Dir(fpath), r, types.CopyToContainerOptions{})
}

func stop(cli *docker.Client, cid string) error {
	vPrintf("stop old container\n")
	return cli.ContainerStop(context.Background(), cid, nil)
}

func start(cli *docker.Client, cid string) error {
	vPrintf("start new container\n")
	return cli.ContainerStart(context.Background(), cid, types.ContainerStartOptions{})
}

func rename(cli *docker.Client, name string) error {
	vPrintf("rename containers\n")
	if err := cli.ContainerRename(context.Background(), name, name+oldContainerSuffix); err != nil {
		return err
	}
	return cli.ContainerRename(context.Background(), name+newContainerSuffix, name)
}

func remove(cli *docker.Client, name string) error {
	vPrintf("remove old container\n")
	return cli.ContainerRemove(context.Background(), name+oldContainerSuffix, types.ContainerRemoveOptions{})
}
