package unlink

import (
	"fmt"
	"github.com/MakeNowJust/heredoc"
	"github.com/cli/cli/v2/api"
	"github.com/cli/cli/v2/internal/ghrepo"
	"github.com/cli/cli/v2/pkg/cmd/project/shared/client"
	"github.com/cli/cli/v2/pkg/cmd/project/shared/queries"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/spf13/cobra"
	"net/http"
	"strconv"
)

type unlinkOpts struct {
	number    int32
	owner     string
	repo      string
	team      string
	projectID string
	repoID    string
	teamID    string
	format    string
	exporter  cmdutil.Exporter
}

type unlinkConfig struct {
	httpClient func() (*http.Client, error)
	client     *queries.Client
	opts       unlinkOpts
	io         *iostreams.IOStreams
}

func NewCmdUnlink(f *cmdutil.Factory, runF func(config unlinkConfig) error) *cobra.Command {
	opts := unlinkOpts{}
	linkCmd := &cobra.Command{
		Short: "Unlink a project from a repository or a team",
		Use:   "unlink [<number>] [flag]",
		Example: heredoc.Doc(`
			# unlink monalisa's project 1 from her repository "my_repo"
			gh project unlink 1 --owner monalisa --repo my_repo

			# unlink monalisa's organization's project 1 from her team "my_team"
			gh project unlink 1 --owner my_organization --team my_team
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := client.New(f)
			if err != nil {
				return err
			}

			if len(args) == 1 {
				num, err := strconv.ParseInt(args[0], 10, 32)
				if err != nil {
					return cmdutil.FlagErrorf("invalid number: %v", args[0])
				}
				opts.number = int32(num)
			}

			config := unlinkConfig{
				httpClient: f.HttpClient,
				client:     client,
				opts:       opts,
				io:         f.IOStreams,
			}

			if err := cmdutil.MutuallyExclusive("specify only one of `--repo` or `--team`", opts.repo != "", opts.team != ""); err != nil {
				return err
			}
			if err := cmdutil.MutuallyExclusive("specify either `--repo` or `--team`", opts.repo == "", opts.team == ""); err != nil {
				return err
			}

			// allow testing of the command without actually running it
			if runF != nil {
				return runF(config)
			}
			return runUnlink(config)
		},
	}

	linkCmd.Flags().StringVar(&opts.owner, "owner", "", "Login of the owner. Use \"@me\" for the current user.")
	linkCmd.Flags().StringVarP(&opts.repo, "repo", "R", "", "The repository to be unlinked from this project")
	linkCmd.Flags().StringVarP(&opts.team, "team", "T", "", "The team to be unlinked from this project")
	cmdutil.AddFormatFlags(linkCmd, &opts.exporter)

	return linkCmd
}

func runUnlink(config unlinkConfig) error {
	canPrompt := config.io.CanPrompt()
	owner, err := config.client.NewOwner(canPrompt, config.opts.owner)
	if err != nil {
		return err
	}

	project, err := config.client.NewProject(canPrompt, owner, config.opts.number, false)
	if err != nil {
		return err
	}
	config.opts.projectID = project.ID

	httpClient, err := config.httpClient()
	if err != nil {
		return err
	}
	c := api.NewClientFromHTTP(httpClient)

	if config.opts.repo != "" {
		return unlinkRepo(c, owner, config)
	} else if config.opts.team != "" {
		return unlinkTeam(c, owner, config)
	}
	return nil
}

func unlinkRepo(c *api.Client, owner *queries.Owner, config unlinkConfig) error {
	repo, err := api.GitHubRepo(c, ghrepo.New(owner.Login, config.opts.repo))
	if err != nil {
		return err
	}
	config.opts.repoID = repo.ID

	result, err := config.client.UnlinkProjectFromRepository(config.opts.projectID, config.opts.repoID)
	if err != nil {
		return err
	}

	if config.opts.exporter != nil {
		return config.opts.exporter.Write(config.io, result)
	}
	return printResults(config, result.URL)
}

func unlinkTeam(c *api.Client, owner *queries.Owner, config unlinkConfig) error {
	team, err := api.OrganizationTeam(c, ghrepo.New(owner.Login, ""), config.opts.team)
	if err != nil {
		return err
	}
	config.opts.teamID = team.ID

	result, err := config.client.UnlinkProjectFromTeam(config.opts.projectID, config.opts.teamID)
	if err != nil {
		return err
	}

	if config.opts.exporter != nil {
		return config.opts.exporter.Write(config.io, result)
	}
	return printResults(config, result.URL)
}

func printResults(config unlinkConfig, url string) error {
	if !config.io.IsStdoutTTY() {
		return nil
	}

	_, err := fmt.Fprintf(config.io.Out, "%s\n", url)
	return err
}
