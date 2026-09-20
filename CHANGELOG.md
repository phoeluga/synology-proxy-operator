# Changelog

## [0.0.9](https://github.com/phoeluga/synology-proxy-operator/compare/v0.0.8...v0.0.9) (2026-09-17)


### Features

* derive source host from Ingress spec.rules when unset ([#40](https://github.com/phoeluga/synology-proxy-operator/issues/40)) ([44b2897](https://github.com/phoeluga/synology-proxy-operator/commit/44b28972c2496df75404bd37e427eda6745be2b0))


### Bug Fixes

* bump golang.org/x/net, x/text, x/sys to patch known CVEs ([#41](https://github.com/phoeluga/synology-proxy-operator/issues/41)) ([76225da](https://github.com/phoeluga/synology-proxy-operator/commit/76225daff2338b6aec5acba3ab2a8a781c3f9a3f))
* replace deprecated controller-runtime scheme.Builder ([3886995](https://github.com/phoeluga/synology-proxy-operator/commit/3886995a56898f3325d3be13d38ef267443f0897))
* use idiomatic naming, correct doc comment, add test coverage ([69a2acb](https://github.com/phoeluga/synology-proxy-operator/commit/69a2acb06832e1002ef664dfb03ed6e2da8ab88f))


### Dependencies

* **deps:** bump actions/checkout from 6 to 7 ([3ecceea](https://github.com/phoeluga/synology-proxy-operator/commit/3ecceea8f9643834611e4bd0326e946a55752627))
* **deps:** bump actions/checkout from 6 to 7 ([35ae660](https://github.com/phoeluga/synology-proxy-operator/commit/35ae660b151dbd9a73df301659b158a2525dac24))
* **deps:** bump actions/setup-go from 6 to 7 ([8ccc1d0](https://github.com/phoeluga/synology-proxy-operator/commit/8ccc1d0d82df3e5752a14a3279e490e5a29dd298))
* **deps:** bump actions/setup-go from 6 to 7 ([afadc0e](https://github.com/phoeluga/synology-proxy-operator/commit/afadc0ecf01a5a056a79cf30a62091b92af887cf))
* **deps:** bump github.com/go-logr/logr from 1.4.3 to 1.4.4 ([390a9f1](https://github.com/phoeluga/synology-proxy-operator/commit/390a9f13914fb907b6b9bf3e84a56100f0c4bb3c))
* **deps:** bump github.com/go-logr/logr from 1.4.3 to 1.4.4 ([fabb56e](https://github.com/phoeluga/synology-proxy-operator/commit/fabb56e037d877768d7177eac17be03eace4dc10))
* **deps:** bump golang from 1.26-alpine to 1.27-alpine ([6208088](https://github.com/phoeluga/synology-proxy-operator/commit/62080884703f5fbc4a384a59ae10a0d363fc4c41))
* **deps:** bump golang from 1.26-alpine to 1.27-alpine ([67a5212](https://github.com/phoeluga/synology-proxy-operator/commit/67a5212e3a4505c7a644f29dbc84447cdb40d0c8))
* **deps:** bump googleapis/release-please-action from 4.4.0 to 5.0.0 ([#29](https://github.com/phoeluga/synology-proxy-operator/issues/29)) ([e9decc4](https://github.com/phoeluga/synology-proxy-operator/commit/e9decc4d4de2cdce706fdc520b95854ad8279199))
* **deps:** bump imjasonh/setup-crane from 0.5 to 0.7 ([0db7166](https://github.com/phoeluga/synology-proxy-operator/commit/0db7166c62227069b29eabf9b7bf1900a5f9b0e5))
* **deps:** bump imjasonh/setup-crane from 0.5 to 0.7 ([7f4328f](https://github.com/phoeluga/synology-proxy-operator/commit/7f4328f2619a43120f2a3495276868392ad95961))
* **deps:** bump oras-project/setup-oras from 1.2.4 to 2.0.0 ([#25](https://github.com/phoeluga/synology-proxy-operator/issues/25)) ([0bb0134](https://github.com/phoeluga/synology-proxy-operator/commit/0bb0134a540a5fd8149e8e9edfb69b83551f3f87))
* **deps:** bump oras-project/setup-oras from 2.0.0 to 2.0.1 ([1b2ad3d](https://github.com/phoeluga/synology-proxy-operator/commit/1b2ad3df2e756f1741ec79aa406f5309b493c839))
* **deps:** bump oras-project/setup-oras from 2.0.0 to 2.0.1 ([ec00ae6](https://github.com/phoeluga/synology-proxy-operator/commit/ec00ae6b567597d899e9bad843611a58226267cc))
* **deps:** bump the k8s group across 1 directory with 3 updates ([933e98e](https://github.com/phoeluga/synology-proxy-operator/commit/933e98e183ca64a7570a7d47218e5fd874a741f2))
* **deps:** bump the k8s group across 1 directory with 3 updates ([d4807de](https://github.com/phoeluga/synology-proxy-operator/commit/d4807def18c69d2ffde76f9145481ed22dc5e720))

## [0.0.8](https://github.com/phoeluga/synology-proxy-operator/compare/v0.0.7...v0.0.8) (2026-04-08)


### Bug Fixes

* prevent duplicate certificate service entries in DSM; treat DSM error 4154 (already exists) as success on create ([da9722b](https://github.com/phoeluga/synology-proxy-operator/commit/da9722bf47eeea9fbd3d8139b4b2bab4d1a238e9))
* skip re-auth on DSM 5xx HTML responses (e.g. 504 timeout) ([fccd04b](https://github.com/phoeluga/synology-proxy-operator/commit/fccd04bcfdf7f0d45dd25308ded0f79bd5c2b02f))
* unassign certificate from DSM before deleting proxy record ([a824269](https://github.com/phoeluga/synology-proxy-operator/commit/a82426968b502a038ee13a2f3ab8b4e06c5acdab))
* unassign old certificate entry when proxy record is recreated with new UUID ([0e2a984](https://github.com/phoeluga/synology-proxy-operator/commit/0e2a9842e96c444d5aa74373253615407500e738))

## [0.0.7](https://github.com/phoeluga/synology-proxy-operator/compare/v0.0.6...v0.0.7) (2026-04-06)


### Features

* add operator.extraArgs to Helm chart ([c2c09f9](https://github.com/phoeluga/synology-proxy-operator/commit/c2c09f96421d4cc7a378ad891fd39396d5140781))
* suppress glob auto-discovery when manual SPR exists in namespace ([e2a17d2](https://github.com/phoeluga/synology-proxy-operator/commit/e2a17d269e9ef306d2caf5f543d133885905c966))


### Bug Fixes

* add disableAutoDiscoveryIfSPRExists to values.schema.json ([3d05e7e](https://github.com/phoeluga/synology-proxy-operator/commit/3d05e7edf115a88cdf0e6eb7a1d78b7da30abe90))
* adding namespace rules ([a980e7e](https://github.com/phoeluga/synology-proxy-operator/commit/a980e7e724a1135e9bed033f166beb1b5b3c98c4))
* check source namespace for manual SPR, not rule namespace ([af6db06](https://github.com/phoeluga/synology-proxy-operator/commit/af6db063d0737f894462803de1a247a0db1fd8ac))
* correct goimports grouping in sprdiscovery.go ([9460264](https://github.com/phoeluga/synology-proxy-operator/commit/946026405d879b8e495b1df09d216268526b2a24))
* log HTML response body and HTTP status when DSM returns non-JSON ([ad109e7](https://github.com/phoeluga/synology-proxy-operator/commit/ad109e783a1ef6ab10519de04a687baa8561bd55))
* re-enqueue Service/Ingress on SPR deletion; handle HTML DSM responses ([70fc458](https://github.com/phoeluga/synology-proxy-operator/commit/70fc458545b830234616021dcc86db85781be5c3))


### Dependencies

* **deps:** bump actions/create-github-app-token from 1 to 3 ([41538ee](https://github.com/phoeluga/synology-proxy-operator/commit/41538ee86ffce5971438c92eb500bd3cf3e3e742))
* **deps:** bump imjasonh/setup-crane from 0.4 to 0.5 ([41538ee](https://github.com/phoeluga/synology-proxy-operator/commit/41538ee86ffce5971438c92eb500bd3cf3e3e742))

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
