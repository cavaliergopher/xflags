package main

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// runMainVar asks the test binary to run orbital rather than its tests.
// Naming it outside the ORBITAL_ namespace keeps it clear of the
// variables run scrubs, and of the ones orbital itself reads.
const runMainVar = "XFLAGS_TEST_RUN_MAIN"

// TestMain runs orbital when runMainVar is set, and the tests otherwise.
// These tests exercise the real program rather than the command tree it
// builds -- an example whose output has drifted is worse than no example,
// and only running it end to end catches that -- and re-executing the
// test binary is what makes that free: go test has already compiled this
// package, so there is nothing to build and no toolchain to find.
func TestMain(m *testing.M) {
	if os.Getenv(runMainVar) != "" {
		main() // calls os.Exit, so this never returns
	}
	os.Exit(m.Run())
}

// run invokes orbital, as a child process, and returns what it wrote and
// the code it exited with. The environment is scrubbed of the variables orbital and
// its completion hook read, so a developer's own shell cannot change what
// a test sees.
func run(t *testing.T, env []string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(os.Args[0], args...)
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "ORBITAL_") || strings.HasPrefix(kv, "COMP_") {
			continue
		}
		cmd.Env = append(cmd.Env, kv)
	}
	cmd.Env = append(cmd.Env, runMainVar+"=1")
	cmd.Env = append(cmd.Env, env...)

	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("running %v: %v", args, err)
		}
	}
	return outBuf.String(), errBuf.String(), cmd.ProcessState.ExitCode()
}

const actor = "ORBITAL_ACTOR=alice"

func TestOrbital(t *testing.T) {
	for _, tt := range []struct {
		name   string
		args   []string
		env    []string
		code   int
		stdout string
		stderr string
	}{
		{
			name: "StatusReads",
			args: []string{"deploy", "status", "api"}, env: []string{actor},
			stdout: "api: healthy (region=us-west-2)\n",
		},
		{
			name: "LogsTakeUnboundedOperands",
			args: []string{"logs", "api", "web"}, env: []string{actor},
			stdout: "api: showing log lines from the last 10m0s\n" +
				"web: showing log lines from the last 10m0s\n",
		},
		{
			name: "ExecForwardsPastTheTerminator",
			args: []string{"exec", "--service", "api", "--", "echo", "hi", "there"},
			env:  []string{actor},
			// The forwarded words reach the handler unparsed, spaces and
			// all, rather than binding to operands of exec itself.
			stdout: "api: would run: echo hi there\n",
		},
		{
			name: "HiddenCommandStillRuns",
			args: []string{"debug"}, env: []string{actor},
			stdout: "debug: dumping internal state (stub)\n",
		},
		{
			name: "EnvVarSatisfiesRequiredFlag",
			args: []string{"deploy", "status", "api"}, env: []string{"ORBITAL_ACTOR=bob"},
			stdout: "api: healthy (region=us-west-2)\n",
		},
		{
			name: "MissingRequiredFlagIsAUsageError",
			args: []string{"deploy", "status", "api"},
			code: 2,
			stderr: "Argument error: missing required argument: --actor\n" +
				"Usage: orbital deploy status [OPTIONS] SERVICE",
		},
		{
			name: "ValueOutsideChoicesIsAUsageError",
			args: []string{"deploy", "run", "--service", "api", "--version", "abcd123",
				"--env", "bogus"},
			env:    []string{actor},
			code:   2,
			stderr: "Argument error: --env: expected one of: staging, production\n",
		},
		{
			name: "UnknownOptionNamesTheSubcommandDeclaringIt",
			args: []string{"--service", "api"}, env: []string{actor},
			code:   2,
			stderr: "Argument error: unrecognized option: --service (defined by subcommand \"run\")\n",
		},
		{
			name: "HandlerNamesItsOwnExitCode",
			args: []string{"deploy", "run", "--service", "api", "--version", "abcd123",
				"--env", "production"},
			env:    []string{actor},
			code:   4,
			stderr: "Error: refusing to deploy api to production without --confirm\n",
		},
		{
			name: "PlainHandlerErrorExitsOne",
			args: []string{"config", "get", "nope"}, env: []string{actor},
			code:   1,
			stderr: "Error: unknown configuration key: nope\n",
		},
		{
			name: "MissingSubcommandIsAUsageError",
			args: []string{"config"}, env: []string{actor},
			code:   2,
			stderr: "Argument error: missing subcommand\n",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, code := run(t, tt.env, tt.args...)
			if got, want := code, tt.code; got != want {
				t.Errorf("exit code = %d, want %d (stderr: %s)", got, want, stderr)
			}
			if got, want := stdout, tt.stdout; !strings.HasPrefix(got, want) {
				t.Errorf("stdout = %q, want prefix %q", got, want)
			}
			if got, want := stderr, tt.stderr; !strings.HasPrefix(got, want) {
				t.Errorf("stderr = %q, want prefix %q", got, want)
			}
		})
	}
}

// TestOrbitalHelp pins the whole help message. It guards the example, and
// the formatter with it: every section orbital exercises -- imported
// groups, a registered group, environment variables, the description --
// renders here.
func TestOrbitalHelp(t *testing.T) {
	stdout, stderr, code := run(t, nil, "--help")
	if got, want := code, 0; got != want {
		t.Errorf("exit code = %d, want %d", got, want)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	if got, want := stdout, wantHelp; got != want {
		t.Errorf("help output:\n%s\nwant:\n%s", got, want)
	}
}

const wantHelp = `Usage: orbital [OPTIONS] COMMAND

Deploy and operate services on the fleet

Options:
   --actor  Identity performing this action, recorded for the audit trail

Legacy options (deprecated):
   --legacy-metrics-addr  Address the legacy metrics sidecar listens on (superseded by --trace)

Telemetry options:
   --log-level  Set the log verbosity (default: info)
   --trace      Emit a timing trace for every command

Commands:
  config  Read and write orbital's local configuration
  deploy  Deploy and inspect services running on the fleet
  logs    Print recent log lines for one or more services
  exec    Run a one-off command inside a service's container

Environment variables:
  ORBITAL_ACTOR  Identity performing this action, recorded for the audit trail
  ORBITAL_TRACE  Emit a timing trace for every command

orbital is the platform team's command line front end to the
fleet API. Each subcommand below is owned and versioned by the
team named in its help text; orbital itself only assembles
them into one binary.
`

// TestOrbitalCompletion drives the shell completion protocol the way a
// generated script does: the request arrives in the environment and the
// reply is newline-separated directive lines on stdout.
func TestOrbitalCompletion(t *testing.T) {
	for _, tt := range []struct {
		name  string
		words []string
		cword string
		want  string
	}{
		{
			// The SERVICE operand completes from the fleet client, so this
			// covers a dynamic CompleteFunc, not just a static list.
			name:  "DynamicOperandCandidates",
			words: []string{"orbital", "logs", ""}, cword: "2",
			want: "plain,api\nplain,billing\nplain,web\nplain,worker\nnofiles,\n",
		},
		{
			name:  "SubcommandNames",
			words: []string{"orbital", "dep"}, cword: "1",
			want: "plain,deploy\nnofiles,\n",
		},
		{
			name:  "ChoicesCompleteAnOptionValue",
			words: []string{"orbital", "deploy", "run", "--env", ""}, cword: "4",
			want: "plain,production\nplain,staging\nnofiles,\n",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			env := []string{
				"ORBITAL_COMPLETE=bash_complete",
				"COMP_WORDS=" + strings.Join(tt.words, "\n"),
				"COMP_CWORD=" + tt.cword,
			}
			stdout, _, code := run(t, env)
			if got, want := code, 0; got != want {
				t.Errorf("exit code = %d, want %d", got, want)
			}
			if got, want := stdout, tt.want; got != want {
				t.Errorf("reply = %q, want %q", got, want)
			}
		})
	}
}

// TestOrbitalCompletionScript asserts the sourced script names the
// program and the variable its hook re-invokes with.
func TestOrbitalCompletionScript(t *testing.T) {
	stdout, _, code := run(t, []string{"ORBITAL_COMPLETE=bash_source"})
	if got, want := code, 0; got != want {
		t.Errorf("exit code = %d, want %d", got, want)
	}
	for _, want := range []string{
		"source <(ORBITAL_COMPLETE=bash_source orbital 2>/dev/null)",
		"ORBITAL_COMPLETE=bash_complete",
		"complete -F _orbital_complete orbital",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("script does not contain %q:\n%s", want, stdout)
		}
	}
}
