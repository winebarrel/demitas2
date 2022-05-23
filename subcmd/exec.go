package subcmd

import (
	"fmt"
	"time"

	"github.com/winebarrel/demitas2"
	"github.com/winebarrel/demitas2/utils"
)

type ExecCmd struct {
	Profile      string `env:"DMTS_PROFILE" short:"p" required:"" help:"Demitas profile name."`
	Command      string `evn:"DMTS_EXEC_COMMAND" required:"" default:"bash" help:"Command to run on a container."`
	Image        string `env:"DMTS_EXEC_IMAGE" default:"public.ecr.aws/lts/ubuntu:latest" help:"Container image."`
	UseTaskImage bool   `env:"DMTS_EXEC_USE_TASK_IMAGE" help:"Use task definition image."`
	SkipStop     bool   `help:"Skip task stop."`
}

func (cmd *ExecCmd) Run(ctx *demitas2.Context) error {
	image := cmd.Image

	if cmd.UseTaskImage {
		image = ""
	}

	def, err := ctx.DefinitionOpts.Load(cmd.Profile, "sleep infinity", image)

	if err != nil {
		return err
	}

	taskId, interrupted, err := ctx.Ecspresso.RunUntilRunning(def, ctx.DryRun)

	if err != nil {
		return err
	}

	if ctx.DryRun {
		return nil
	}

	if taskId == "" {
		return fmt.Errorf("task ID not found")
	}

	return utils.TrapInt(
		func() error {
			if interrupted {
				return nil
			}

			for {
				err = ctx.Ecs.ExecuteCommand(def.Cluster, taskId, "id")

				if err == nil {
					break
				}

				time.Sleep(1 * time.Second)
			}

			return ctx.Ecs.ExecuteInteractiveCommand(def.Cluster, taskId, cmd.Command)
		},
		func() {
			if cmd.SkipStop {
				fmt.Printf(`ECS task is still running.

Re-login command:
  aws ecs execute-command --cluster %s --task %s --interactive --command %s

Task stop command:
  aws ecs stop-task --cluster %s --task %s
`,
					def.Cluster, taskId, cmd.Command,
					def.Cluster, taskId,
				)

				return
			}

			fmt.Printf("Stopping task: %s\n", taskId)
			ctx.Ecs.StopTask(def.Cluster, taskId)
		})
}
