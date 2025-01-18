# Changelog

## [1.2.0](https://github.com/iandyh/shibuya/compare/v1.1.2...v1.2.0) (2025-01-18)


### Features

* add prefix to differ from main release ([70a38f5](https://github.com/iandyh/shibuya/commit/70a38f574ad5593c78d77456b6a83f735d62f3e4))
* distributed mode p2 release. We will send the metrics from engines to storage via scrapers instead of via controller. ([4a4de0c](https://github.com/iandyh/shibuya/commit/4a4de0c82c68e2ec6cc1d8c5ffb5f7dbc6b45b59))
* Enable engine metrics exposing in the agent ([#112](https://github.com/iandyh/shibuya/issues/112)) ([d7d25ad](https://github.com/iandyh/shibuya/commit/d7d25adcb96451bc33d1d536f5b7017a64e1f4ba))
* introduce release please ([9f33ad0](https://github.com/iandyh/shibuya/commit/9f33ad0c7c22d1063b68fc22f7746e1ce748c86f))
* use some middlewares from chi and make some customisations ([8421859](https://github.com/iandyh/shibuya/commit/8421859e56e5cb005a61102de86f9c6ead207ee9))


### Bug Fixes

* add missing charts ([420cdf9](https://github.com/iandyh/shibuya/commit/420cdf94fa56d13b7bec7ce12dde20d14c1ffc39))
* add missing if ([fc5622c](https://github.com/iandyh/shibuya/commit/fc5622ca1a59ca3dec356039145bac5f6bf15c9c))
* agent should also laod the config for object storage ([7c9f00c](https://github.com/iandyh/shibuya/commit/7c9f00c8788c522489265d4085ce093f02dbf7a1))
* better naming ([12a42de](https://github.com/iandyh/shibuya/commit/12a42de7e83c3e37f0e44a6fff923a5f59e48cfe))
* chart could not be generated due to tagging. Use gh cli directly instead of chart-releaser-action ([#111](https://github.com/iandyh/shibuya/issues/111)) ([8ab71bb](https://github.com/iandyh/shibuya/commit/8ab71bb47ce99c5c4d8e42976bcb277409f1354a))
* exclude scraper in engine details ([d3c302e](https://github.com/iandyh/shibuya/commit/d3c302ea420b55c1122ca01685603523ef8c247e))
* finish config factoring ([70a058f](https://github.com/iandyh/shibuya/commit/70a058facc008ba8c9d0a66be85f2f11f1a50a13))
* fix delete api ([cfc5126](https://github.com/iandyh/shibuya/commit/cfc5126d5643115d36f8b5a317b7cb5e5967d089))
* fix metric dashboard repo url ([#131](https://github.com/iandyh/shibuya/issues/131)) ([161c5e6](https://github.com/iandyh/shibuya/commit/161c5e64208dcc5637aaf899d1b81298ee40adc3))
* fix tests ([beff155](https://github.com/iandyh/shibuya/commit/beff1554b8f44b85b99bf7100d319c9704f167e1))
* give more resources to local engines ([59fc446](https://github.com/iandyh/shibuya/commit/59fc44627bb7c4964962d0c385a4282c5a602f50))
* ingress controller is renamed to coordinator and it is also built with github action ([286abbf](https://github.com/iandyh/shibuya/commit/286abbf2327630a078ee2c5afb9f55cda08bf489))
* local controller is a wrong binary ([8c729e4](https://github.com/iandyh/shibuya/commit/8c729e4e4bcb0a6b6e1f0af310d9c2b0653ca21f))
* make the sa token management more flexible ([476f545](https://github.com/iandyh/shibuya/commit/476f545c09add0a83ae4800bca3d615980082221))
* only build the image when it is a release ([d8fc0a1](https://github.com/iandyh/shibuya/commit/d8fc0a1496f591d6c9254460010b28e3187bf5d8))
* prevent fork polutting the officical release registry ([77906e5](https://github.com/iandyh/shibuya/commit/77906e5140365321eb881d7c1edf2db1a94e1ae9))
* remove execess RBAC ([9c1e9d5](https://github.com/iandyh/shibuya/commit/9c1e9d53c076335683978bf4ba951b732dd987b1))
* remove logging ([#129](https://github.com/iandyh/shibuya/issues/129)) ([83f9353](https://github.com/iandyh/shibuya/commit/83f93539c5b579ce1448fbfa752e254e7c8a2d8e))
* replace gcr image and upgrade go to 1.23 ([317ec90](https://github.com/iandyh/shibuya/commit/317ec90639520c7466f66cc794cd086807451ca6))
* replace master with control ([5c9234e](https://github.com/iandyh/shibuya/commit/5c9234e6e89bd995c1050196277e49a797db5c88))
* should use release action from googleapi repo ([c38a4bb](https://github.com/iandyh/shibuya/commit/c38a4bb2aaeb172a4d1e44296715d950724f5008))
* upgrade go version to latest ([#133](https://github.com/iandyh/shibuya/issues/133)) ([a8dbe3b](https://github.com/iandyh/shibuya/commit/a8dbe3b45b49f4fef2db05b48f5cd60d7a295467))
* upgrade underscorejs to v1.13.7 ([8b8879a](https://github.com/iandyh/shibuya/commit/8b8879a407632a19769cbbf18afc8e04136bfbd4))
* we should switch to histogram for aggregation ([018c0c9](https://github.com/iandyh/shibuya/commit/018c0c982cac4ff3e489121b735e87e149fd74b5))
* when no_auth is enabled, isadmin should be true ([b25ac09](https://github.com/iandyh/shibuya/commit/b25ac091132ecc93039d6c1bff58bafb7eafbaf9))
* wrong metrics dashboard image at the local ([0b39cd0](https://github.com/iandyh/shibuya/commit/0b39cd000ad805fc2c5a4414dea654e155c3366a))
* wrong metrics dashboard image at the local ([a980b8a](https://github.com/iandyh/shibuya/commit/a980b8a1fa0cdc6f49abdaf5566e835785277181))
* wrong tag name ([4b33f75](https://github.com/iandyh/shibuya/commit/4b33f7506cf2863665052650b3744ec8505adf1e))

## [1.1.2](https://github.com/rakutentech/shibuya/compare/v1.1.1...v1.1.2) (2024-12-16)


### Bug Fixes

* fix metric dashboard repo url ([#131](https://github.com/rakutentech/shibuya/issues/131)) ([161c5e6](https://github.com/rakutentech/shibuya/commit/161c5e64208dcc5637aaf899d1b81298ee40adc3))

## [1.1.1](https://github.com/rakutentech/shibuya/compare/v1.1.0...v1.1.1) (2024-12-16)


### Bug Fixes

* remove logging ([#129](https://github.com/rakutentech/shibuya/issues/129)) ([83f9353](https://github.com/rakutentech/shibuya/commit/83f93539c5b579ce1448fbfa752e254e7c8a2d8e))

## [1.1.0](https://github.com/rakutentech/shibuya/compare/v1.0.0...v1.1.0) (2024-10-01)


### Features

* Enable engine metrics exposing in the agent ([#112](https://github.com/rakutentech/shibuya/issues/112)) ([d7d25ad](https://github.com/rakutentech/shibuya/commit/d7d25adcb96451bc33d1d536f5b7017a64e1f4ba))

## 1.0.0 (2024-08-30)


### Features

* add prefix to differ from main release ([70a38f5](https://github.com/rakutentech/shibuya/commit/70a38f574ad5593c78d77456b6a83f735d62f3e4))
* introduce release please ([9f33ad0](https://github.com/rakutentech/shibuya/commit/9f33ad0c7c22d1063b68fc22f7746e1ce748c86f))


### Bug Fixes

* add missing charts ([420cdf9](https://github.com/rakutentech/shibuya/commit/420cdf94fa56d13b7bec7ce12dde20d14c1ffc39))
* add missing if ([fc5622c](https://github.com/rakutentech/shibuya/commit/fc5622ca1a59ca3dec356039145bac5f6bf15c9c))
* better naming ([12a42de](https://github.com/rakutentech/shibuya/commit/12a42de7e83c3e37f0e44a6fff923a5f59e48cfe))
* chart could not be generated due to tagging. Use gh cli directly instead of chart-releaser-action ([#111](https://github.com/rakutentech/shibuya/issues/111)) ([8ab71bb](https://github.com/rakutentech/shibuya/commit/8ab71bb47ce99c5c4d8e42976bcb277409f1354a))
* only build the image when it is a release ([d8fc0a1](https://github.com/rakutentech/shibuya/commit/d8fc0a1496f591d6c9254460010b28e3187bf5d8))
* prevent fork polutting the officical release registry ([77906e5](https://github.com/rakutentech/shibuya/commit/77906e5140365321eb881d7c1edf2db1a94e1ae9))
* should use release action from googleapi repo ([c38a4bb](https://github.com/rakutentech/shibuya/commit/c38a4bb2aaeb172a4d1e44296715d950724f5008))
* wrong tag name ([4b33f75](https://github.com/rakutentech/shibuya/commit/4b33f7506cf2863665052650b3744ec8505adf1e))
