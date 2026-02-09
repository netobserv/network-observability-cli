## RPM pre-fetching for Konflux

1. Doc reference is here: https://konflux.pages.redhat.com/docs/users/building/prefetching-dependencies.html#rpm-walkthrough

2. How it's currently done:

Tool rpm-lockfile-prototype is downloaded as a Docker image via:

```
curl https://raw.githubusercontent.com/konflux-ci/rpm-lockfile-prototype/refs/heads/main/Containerfile \
   | $(OCI_BIN) build -t localhost/rpm-lockfile-prototype -
```

Then, ubi.repo was created out of the current base image (which means, we need to do it again every time the base image changes)

```
BASE_IMAGE=registry.access.redhat.com/ubi9/ubi-minimal:9.7-1770238273
podman run -it $BASE_IMAGE cat /etc/yum.repos.d/ubi.repo > rpm-prefetching/ubi.repo
sed -i 's/ubi-9-codeready-builder/codeready-builder-for-ubi-9-$basearch/' rpm-prefetching/ubi.repo
sed -i 's/\[ubi-9/[ubi-9-for-$basearch/' rpm-prefetching/ubi.repo
```

Finally, run rpm-lockfile-prototype:

```
podman run --privileged --rm -v ./rpm-prefetching:/work localhost/rpm-lockfile-prototype:latest --outfile=rpms.lock.yaml --image $BASE_IMAGE rpms.in.yaml
```

3. Automate all this!
