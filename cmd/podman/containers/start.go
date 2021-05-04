package containers

import (
	"fmt"
	"os"

	"github.com/containers/podman/v3/cmd/podman/common"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/cmd/podman/utils"
	"github.com/containers/podman/v3/cmd/podman/validate"
	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	startDescription = `Starts one or more containers.  The container name or ID can be used.`
	startCommand     = &cobra.Command{
		Use:               "start [options] CONTAINER [CONTAINER...]",
		Short:             "Start one or more containers",
		Long:              startDescription,
		RunE:              start,
		Args:              validateStart,
		ValidArgsFunction: common.AutocompleteContainersStartable,
		Example: `podman start --latest
  podman start 860a4b231279 5421ab43b45
  podman start --interactive --attach imageID`,
	}

	containerStartCommand = &cobra.Command{
		Use:               startCommand.Use,
		Short:             startCommand.Short,
		Long:              startCommand.Long,
		RunE:              startCommand.RunE,
		Args:              startCommand.Args,
		ValidArgsFunction: startCommand.ValidArgsFunction,
		Example: `podman container start --latest
  podman container start 860a4b231279 5421ab43b45
  podman container start --interactive --attach imageID`,
	}
)

var (
	startOptions entities.ContainerStartOptions
)

func startFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	flags.BoolVarP(&startOptions.Attach, "attach", "a", false, "Attach container's STDOUT and STDERR")

	detachKeysFlagName := "detach-keys"
	flags.StringVar(&startOptions.DetachKeys, detachKeysFlagName, containerConfig.DetachKeys(), "Select the key sequence for detaching a container. Format is a single character `[a-Z]` or a comma separated sequence of `ctrl-<value>`, where `<value>` is one of: `a-z`, `@`, `^`, `[`, `\\`, `]`, `^` or `_`")
	_ = cmd.RegisterFlagCompletionFunc(detachKeysFlagName, common.AutocompleteDetachKeys)

	flags.BoolVarP(&startOptions.Interactive, "interactive", "i", false, "Keep STDIN open even if not attached")
	flags.BoolVar(&startOptions.SigProxy, "sig-proxy", false, "Proxy received signals to the process (default true if attaching, false otherwise)")

	flags.BoolVar(&startOptions.All, "all", false, "Start all containers regardless of their state or configuration")

	if registry.IsRemote() {
		_ = flags.MarkHidden("sig-proxy")
	}
}
func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: startCommand,
	})
	startFlags(startCommand)
	validate.AddLatestFlag(startCommand, &startOptions.Latest)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: containerStartCommand,
		Parent:  containerCmd,
	})
	startFlags(containerStartCommand)
	validate.AddLatestFlag(containerStartCommand, &startOptions.Latest)
}

func validateStart(cmd *cobra.Command, args []string) error {
	if len(args) == 0 && !startOptions.Latest && !startOptions.All {
		return errors.New("start requires at least one argument")
	}
	if len(args) > 0 && startOptions.Latest {
		return errors.Errorf("--latest and containers cannot be used together")
	}
	if len(args) > 1 && startOptions.Attach {
		return errors.Errorf("you cannot start and attach multiple containers at once")
	}
	if (len(args) > 0 || startOptions.Latest) && startOptions.All {
		return errors.Errorf("either start all containers or the container(s) provided in the arguments")
	}
	if startOptions.Attach && startOptions.All {
		return errors.Errorf("you cannot start and attach all containers at once")
	}
	return nil
}

func start(cmd *cobra.Command, args []string) error {
	var errs utils.OutputErrors
	sigProxy := startOptions.SigProxy || startOptions.Attach
	if cmd.Flag("sig-proxy").Changed {
		sigProxy = startOptions.SigProxy
	}
	startOptions.SigProxy = sigProxy

	if sigProxy && !startOptions.Attach {
		return errors.Wrapf(define.ErrInvalidArg, "you cannot use sig-proxy without --attach")
	}
	if startOptions.Attach {
		startOptions.Stdin = os.Stdin
		startOptions.Stderr = os.Stderr
		startOptions.Stdout = os.Stdout
	}

	responses, err := registry.ContainerEngine().ContainerStart(registry.GetContext(), args, startOptions)
	if err != nil {
		return err
	}

	for _, r := range responses {
		if r.Err == nil {
			if startOptions.Attach {
				// Implement the exitcode when the only one container is enabled attach
				registry.SetExitCode(r.ExitCode)
			} else {
				fmt.Println(r.RawInput)
			}
		} else {
			errs = append(errs, r.Err)
		}
	}

	return errs.PrintErrors()
}
