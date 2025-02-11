## Releasing NetObserv CLI

### Tagging & creating a draft release

This is the process of releasing the NetObserv CLI on GitHub. First, tag from the release branch that you want to publish (make sure you're up to date):

```bash
git fetch upstream
git reset --hard upstream/(release branch)
version="v0.0.6"
git tag -a "$version" -m "$version"
git push upstream --tags
```

When the tag is pushed, a release action is triggered on GitHub: https://github.com/netobserv/network-observability-cli/actions/workflows/release.yml.

When the job completes, you should see a new draft release in https://github.com/netobserv/network-observability-cli/releases.

### Krew

If you haven't already, fork and clone the krew-index repo from https://github.com/kubernetes-sigs/krew-index.

From there, you'll find the NetObserv plugin info in `plugins/netobserv.yaml`.

Copy and paste the YAML snippet provided in draft release on GitHub, into that `netobserv.yaml` file.

To test it, first download the tgz archive from the GitHub release (see the `netobserv-cli.tar.gz` link under "Assets").

Then:

```bash
# uninstall any previous version of the plugin
kubectl krew uninstall netobserv
# reinstall using the current manifest and archive
kubectl krew install --manifest=plugins/netobserv.yaml --archive=/path/to/netobserv-cli.tar.gz
kubectl netobserv version
# output: Netobserv CLI version <the new version>

# smoke-test on a live cluster
kubectl netobserv flows
```

NB: The process to publish a plugin update is also documented in https://krew.sigs.k8s.io/docs/developer-guide/release/updating-plugins/.

### Publish the GitHub release

When tests are OK, edit the draft release in GitHub:
- Remove the text template
- Auto-generate the release note
- Check the "Set as the latest release" box
- Click Publish

### Krew again

Finally, commit the YAML changes and open a pull request:

```bash
git commit -a -s -m "Bump netobserv $version"
git push origin HEAD:bump-$version
```

Note: the first time, you may need to sign the CLA for the Linux Foundation / CNCF. Check your PR for any additional step to take.
