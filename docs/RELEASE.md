## Releasing NetObserv CLI

### Tagging & creating a draft release

This is the process of releasing the NetObserv CLI on GitHub. First, tag from the release branch that you want to publish (make sure you're up to date):

```bash
git fetch upstream
git reset --hard upstream/(release branch)
version="v0.0.9"
git tag -a "$version" -m "$version"
git push upstream --tags
```

When the tag is pushed, a release action is triggered on GitHub: https://github.com/netobserv/network-observability-cli/actions/workflows/release.yml.

When the job completes, you should see a new draft release in https://github.com/netobserv/network-observability-cli/releases.

### Publish the GitHub release

Edit the draft release in GitHub:
- Remove the text template
- Auto-generate the release note
- Check the "Set as the latest release" box
- Click Publish

### Krew

The krew-release-bot is triggered automatically when the release is published.
