# Changelog

## [0.0.7](https://github.com/phoeluga/synology-proxy-operator/compare/v0.0.6...v0.0.7) (2026-04-05)


### Features

* suppress glob auto-discovery when manual SPR exists in namespace ([e2a17d2](https://github.com/phoeluga/synology-proxy-operator/commit/e2a17d269e9ef306d2caf5f543d133885905c966))


### Bug Fixes

* add disableAutoDiscoveryIfSPRExists to values.schema.json ([3d05e7e](https://github.com/phoeluga/synology-proxy-operator/commit/3d05e7edf115a88cdf0e6eb7a1d78b7da30abe90))
* adding namespace rules ([a980e7e](https://github.com/phoeluga/synology-proxy-operator/commit/a980e7e724a1135e9bed033f166beb1b5b3c98c4))
* check source namespace for manual SPR, not rule namespace ([af6db06](https://github.com/phoeluga/synology-proxy-operator/commit/af6db063d0737f894462803de1a247a0db1fd8ac))
* correct goimports grouping in sprdiscovery.go ([9460264](https://github.com/phoeluga/synology-proxy-operator/commit/946026405d879b8e495b1df09d216268526b2a24))

## [0.0.6](https://github.com/phoeluga/synology-proxy-operator/compare/v0.0.5...v0.0.6) (2026-04-03)


### Bug Fixes

* add missing podSecurityContext and securityContext properties to values schema ([46b0870](https://github.com/phoeluga/synology-proxy-operator/commit/46b08703b916f8037bbdcec355c0ad1afa282af7))
* distributed SPR namespaces, correct print columns, deepcopy generation ([6061581](https://github.com/phoeluga/synology-proxy-operator/commit/60615814a1ba9e0bb9ffe53215428176e0207c43))
* replace yq with python in schema-drift check ([84f439f](https://github.com/phoeluga/synology-proxy-operator/commit/84f439fa37b5de8785a104621cf6bc5501b089c4))

## [0.0.5](https://github.com/phoeluga/synology-proxy-operator/compare/v0.0.4...v0.0.5) (2026-04-02)


### Bug Fixes

* restructure CI/CD workflows, sign Helm chart, fix ArtifactHub verification ([510bd3e](https://github.com/phoeluga/synology-proxy-operator/commit/510bd3e830e92836f530300132041415f34ce363))
* Updated CI execution check ([f53e516](https://github.com/phoeluga/synology-proxy-operator/commit/f53e516b0c19b1812c88113184a7c55d2f3c1ef2))
* Updated CI/CD naming ([ab5d955](https://github.com/phoeluga/synology-proxy-operator/commit/ab5d955812a0be439550228bb137272df4e78a94))

## [0.0.4](https://github.com/phoeluga/synology-proxy-operator/compare/v0.0.3...v0.0.4) (2026-04-01)


### Features

* sign releases with cosign, upgrade Go 1.25, add ArtifactHub verification workflow ([#5](https://github.com/phoeluga/synology-proxy-operator/issues/5)) ([47989e1](https://github.com/phoeluga/synology-proxy-operator/commit/47989e136d15e7ec8cd982a34f17af92c7998653))


### Bug Fixes

* adding direct chart link to sources ([a838505](https://github.com/phoeluga/synology-proxy-operator/commit/a838505a6e6240f1805f09967a36fe1fa574c7bd))
* sign releases with cosign, upgrade Go 1.25, add ArtifactHub verification workflow ([e36b64a](https://github.com/phoeluga/synology-proxy-operator/commit/e36b64a021431dea3f4072d6fa754d7e9b1dd3dd))
* sign releases with cosign, upgrade Go 1.25, add ArtifactHub verification workflow ([69b27a5](https://github.com/phoeluga/synology-proxy-operator/commit/69b27a533eb017e4d3563d8aa16755f2317f02e0))


### Dependencies

* **deps:** bump golang from 1.25-alpine to 1.26-alpine ([#6](https://github.com/phoeluga/synology-proxy-operator/issues/6)) ([0d701df](https://github.com/phoeluga/synology-proxy-operator/commit/0d701df25f6a6c5e17c9476d5d9f5a1b90e02143))
* **deps:** bump versions ([#12](https://github.com/phoeluga/synology-proxy-operator/issues/12)) ([baaac6f](https://github.com/phoeluga/synology-proxy-operator/commit/baaac6f63eeb661e518591558d87d43953c70b3e))
