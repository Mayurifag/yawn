# Configuration

Config is loaded in this order:

| Source                       | Notes                                                |
| ---------------------------- | ---------------------------------------------------- |
| `~/.config/yawn/config.toml` | User config.                                         |
| `.yawn.toml`                 | Project config, also searched in parent directories. |
| `YAWN_*`                     | Environment variables.                               |
| CLI flags                    | Highest precedence.                                  |

Invalid boolean or integer environment values produce an error. `request_timeout_seconds` must be positive and fit in a
Go `time.Duration`.

Explicit empty string values clear inherited API keys, models, or fallback providers. An empty model selects the
provider's default model. Empty boolean or integer environment values are invalid rather than treated as unset.

When saving an API key, yawn preserves other settings. If it cannot safely edit the TOML layout, it leaves the file
unchanged and asks you to set the key manually. Terminal API-key input is hidden.

## Providers

Supported providers are intentionally small:

| Provider       | Auth                     | Notes                                         |
| -------------- | ------------------------ | --------------------------------------------- |
| `gemini`       | Google AI Studio API key | Default direct API provider.                  |
| `opencode_cli` | Local OpenCode login     | Uses models available in your OpenCode setup. |

OpenCode V2 runs on a private server with tools disabled. Models use `provider/model#low` unless an explicit variant is
configured. Reasoning output is off by default, and no output token limit is set.

## Options

| Key                       | Meaning                                                                                        |
| ------------------------- | ---------------------------------------------------------------------------------------------- |
| `prompt`                  | Commit-message instructions.                                                                   |
| `main_provider`           | Primary AI provider. Default: `gemini`.                                                        |
| `fallback_provider`       | Optional backup provider. Constructed lazily only after primary failure.                       |
| `request_timeout_seconds` | Timeout per provider attempt. Default: `15`.                                                   |
| `auto_stage`              | Stage changes without prompting.                                                               |
| `auto_push`               | Push after committing without prompting; divergent histories still require force-push consent. |
| `push_command`            | Push command. Default: `git push origin HEAD`.                                                 |
| `squash_auto_push`        | Force-push automatically after `yawn squash`, without confirmation.                            |
| `wait_for_ssh_keys`       | Wait for `ssh-add -l` before pushing. Useful with KeePassXC or other SSH-agent unlock flows.   |

## Push commands

`push_command` accepts `git push [options] [remote [branch[:destination]]]`. The source must be `HEAD`, the current
branch name, or its full `refs/heads/` ref. The destination must be one branch. For example:

```toml
push_command = "git push -u upstream HEAD:review"
```

Without a destination refspec, yawn uses the current branch name. Without a remote, it checks
`branch.<name>.pushRemote`, `remote.pushDefault`, and `branch.<name>.remote`, then uses `origin`. A remote with
configured push refspecs requires an explicit branch in `push_command`.

Supported options are `-u`/`--set-upstream`, `-f`/`--force`, `--force-with-lease`, `--force-if-includes`,
`-v`/`--verbose`, `-q`/`--quiet`, `--porcelain`, `--verify`, and `--no-verify`. Force options use yawn's explicit
previewed lease, not a blind force push. Other options, shell quoting, multiple refspecs, tag destinations, multiple
push URLs, and custom lease values are rejected. HTTPS URLs must use a credential helper rather than embedded
credentials.

Push inspection contacts the destination and fetches its tip without changing tracking refs or FETCH_HEAD. If inspection
fails, yawn does not push. The source commit and force-push lease stay fixed across retries. An unpublished branch can
be pushed without configuring an upstream first. Pushes use the named remote, preserving its settings and the remote
name passed to pre-push hooks. A changed push URL requires a new preview. PR lookup only returns open PRs from the push
repository. For a renamed destination branch, PR links use the local branch's parent as the base.

## CLI Flags

| Flag                | Meaning                                |
| ------------------- | -------------------------------------- |
| `--api-key`         | Override the primary provider API key. |
| `--auto-stage`      | Stage all changes without prompting.   |
| `--auto-push`       | Push after commit without prompting.   |
| `--generate-config` | Print the default config template.     |
| `--version`         | Print version information.             |
| `squash --base REF` | Squash since the merge-base with REF.  |

Squash uses the recorded branch-creation parent by default. If that evidence is missing or the parent no longer exists,
specify `--base`. It does not infer a parent from arbitrary ancestor branches. Squashing the default branch or a
detached HEAD is refused.
