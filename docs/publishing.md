# Publishing images later

Local builds and ordinary CI runs never push images. A published GitHub release
under the reactor2k3 account triggers the GHCR job after verification succeeds.

## Optional Docker Hub publication

Docker Hub is **disabled by default**. Its login and image target require the
GitHub repository variable `DOCKERHUB_ENABLED` to be exactly `true`.

When publication is explicitly wanted later:

1. Create a repository in your Docker Hub account.
2. Set GitHub repository variables `DOCKERHUB_USERNAME` and `DOCKERHUB_IMAGE`
   (full `namespace/repository`, for example `reactor2k3/ddns-updater`).
3. Add a Docker Hub personal access token with the required read/write permission
   as the GitHub Actions secret `DOCKERHUB_TOKEN`. Enter it directly into GitHub's
   secret settings; never put it in source code or chat.
4. Explicitly set `DOCKERHUB_ENABLED=true` and publish the chosen release.

The same build is tagged for GHCR and Docker Hub, targeting AMD64 and ARM64.
Prereleases do not replace `latest`. Set `DOCKERHUB_ENABLED=false` to disable
Docker Hub again. No Docker Hub repository or credentials are created by this
configuration, and nothing has been uploaded merely by preparing it.

See [Docker access tokens](https://docs.docker.com/security/access-tokens/) and
[GitHub Actions publishing](https://docs.docker.com/guides/angular/configure-github-actions/).
