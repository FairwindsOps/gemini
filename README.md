<div align="center">
  <img src="/logo.png" alt="Gemini Logo" />
  <br>

  [![Version][version-image]][version-link] [![CircleCI][circleci-image]][circleci-link] [![Go Report Card][goreport-image]][goreport-link] [![Codecov][codecov-image]][codecov-link]
</div>

[version-image]: https://img.shields.io/static/v1.svg?label=Version&message=0.0.1&color=239922
[version-link]: https://github.com/FairwindsOps/gemini

[goreport-image]: https://goreportcard.com/badge/github.com/FairwindsOps/gemini
[goreport-link]: https://goreportcard.com/report/github.com/FairwindsOps/gemini

[circleci-image]: https://circleci.com/gh/FairwindsOps/gemini.svg?style=svg
[circleci-link]: https://circleci.com/gh/FairwindsOps/gemini

[codecov-image]: https://codecov.io/gh/FairwindsOps/gemini/branch/master/graph/badge.svg?token=7C20K7SYNR
[codecov-link]: https://codecov.io/gh/FairwindsOps/gemini

Gemini is a Kubernetes CRD and operator for managing `VolumeSnapshots`. This allows you
to back up your `PersistentVolumes` on a regular schedule, retire old backups, and restore
backups with minimal downtime.

> Note: Like the VolumeSnapshot API it builds on, Gemini is **currently in beta**.


<!-- Begin boilerplate -->
## Join the Fairwinds Open Source Community

The goal of the Fairwinds Community is to exchange ideas, influence the open source roadmap,
and network with fellow Kubernetes users.
[Chat with us on Slack](https://join.slack.com/t/fairwindscommunity/shared_invite/zt-e3c6vj4l-3lIH6dvKqzWII5fSSFDi1g)
[join the user group](https://www.fairwinds.com/open-source-software-user-group) to get involved!

<a href="https://www.fairwinds.com/t-shirt-offer?utm_source=gemini&utm_medium=gemini&utm_campaign=gemini-tshirt">
  <img src="https://www.fairwinds.com/hubfs/Doc_Banners/Fairwinds_OSS_User_Group_740x125_v6.png" alt="Love Fairwinds Open Source? Share your business email and job title and we'll send you a free Fairwinds t-shirt!" />
</a>

## Other Projects from Fairwinds

Enjoying Gemini? Check out some of our other projects:
* [Polaris](https://github.com/FairwindsOps/Polaris) - Audit, enforce, and build policies for Kubernetes resources, including over 20 built-in checks for best practices
* [Goldilocks](https://github.com/FairwindsOps/Goldilocks) - Right-size your Kubernetes Deployments by compare your memory and CPU settings against actual usage
* [Pluto](https://github.com/FairwindsOps/Pluto) - Detect Kubernetes resources that have been deprecated or removed in future versions
* [Nova](https://github.com/FairwindsOps/Nova) - Check to see if any of your Helm charts have updates available
* [rbac-manager](https://github.com/FairwindsOps/rbac-manager) - Simplify the management of RBAC in your Kubernetes clusters
