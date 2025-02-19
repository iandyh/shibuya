# Changelog

## [1.2.0](https://github.com/iandyh/shibuya/compare/v1.1.2...v1.2.0) (2025-02-19)


### Features

* add certs to coordinator ([ba21608](https://github.com/iandyh/shibuya/commit/ba21608e2fc6592eab637b11156a852622fe2d4e))
* add collectionstatus in the api ([8f9441e](https://github.com/iandyh/shibuya/commit/8f9441e378f5b6a9be1b320f8bfad1932fc31b19))
* add prefix to differ from main release ([70a38f5](https://github.com/iandyh/shibuya/commit/70a38f574ad5593c78d77456b6a83f735d62f3e4))
* agent http endpoints have token based auth ([2bc3860](https://github.com/iandyh/shibuya/commit/2bc3860c4df92f825a30d0aee9575c4b6b0422e5))
* all exposed apis will have keys ([e6dc745](https://github.com/iandyh/shibuya/commit/e6dc745f306d4c5da8d49468a70fc44cfa866312))
* auth with token in api. As a result, we also no longer need to disable auth in local ([7026606](https://github.com/iandyh/shibuya/commit/702660629fa9f98390264a47cb5ea8520cf090b9))
* client support ([4af3b84](https://github.com/iandyh/shibuya/commit/4af3b84f0e4b196b2da10859da0f8052e90c472e))
* containers are running as non-root ([5247ea6](https://github.com/iandyh/shibuya/commit/5247ea611db30bb002a72ac2e378d2b5085ecd20))
* distributed mode p2 release. We will send the metrics from engines to storage via scrapers instead of via controller. ([4a4de0c](https://github.com/iandyh/shibuya/commit/4a4de0c82c68e2ec6cc1d8c5ffb5f7dbc6b45b59))
* Enable engine metrics exposing in the agent ([#112](https://github.com/iandyh/shibuya/issues/112)) ([d7d25ad](https://github.com/iandyh/shibuya/commit/d7d25adcb96451bc33d1d536f5b7017a64e1f4ba))
* engine recovers itself when the coordinator is down ([0c89393](https://github.com/iandyh/shibuya/commit/0c8939384ff54dd92a0d5d794d0905811ada7fbe))
* introduce release please ([9f33ad0](https://github.com/iandyh/shibuya/commit/9f33ad0c7c22d1063b68fc22f7746e1ce748c86f))
* login with google ([120e32b](https://github.com/iandyh/shibuya/commit/120e32bff31deac79de57cf6a9f92160df22932c))
* make the sync period to 2 seconds ([123eb88](https://github.com/iandyh/shibuya/commit/123eb88e63caecd3b32555c81f5773e76a3eb208))
* metrics goes through api layer ([e071168](https://github.com/iandyh/shibuya/commit/e071168d3b1568495f27ac057e613edd692d3e66))
* new way of communication between shibuya controller and the engines. Coordinator takes most of the work away. ([5f09e31](https://github.com/iandyh/shibuya/commit/5f09e31f83289eaa201f2c5698dff78ce41940e3))
* pubsub also has password based auth ([1b09e30](https://github.com/iandyh/shibuya/commit/1b09e30e686d4ed33320fb0f9a76a7ee7219d654))
* separate the service account for coordinator and scraper ([03af5f9](https://github.com/iandyh/shibuya/commit/03af5f9014c6990be26298bbbeedaf76281b89ea))
* support locust as second load generator ([680ca0b](https://github.com/iandyh/shibuya/commit/680ca0b975e91814cf7ac812fb25f85cf1a86e3c))
* use some middlewares from chi and make some customisations ([8421859](https://github.com/iandyh/shibuya/commit/8421859e56e5cb005a61102de86f9c6ead207ee9))


### Bug Fixes

* add missing charts ([420cdf9](https://github.com/iandyh/shibuya/commit/420cdf94fa56d13b7bec7ce12dde20d14c1ffc39))
* add missing if ([fc5622c](https://github.com/iandyh/shibuya/commit/fc5622ca1a59ca3dec356039145bac5f6bf15c9c))
* add missing label for scraper ([28ab940](https://github.com/iandyh/shibuya/commit/28ab9406e62ef100c9c98cf4093460412196aec0))
* add missing rbac for scraper ([6b9d096](https://github.com/iandyh/shibuya/commit/6b9d0965ec856120cc67ec0cc95767d92a653ca0))
* add missing return ([82a308c](https://github.com/iandyh/shibuya/commit/82a308c6a7b48c0de8df2ae6185572b286a6c9a5))
* add missing watch for scraper ([0ddfca5](https://github.com/iandyh/shibuya/commit/0ddfca55f381f4ce1d6f34b18c247a60c697f2cc))
* add the missing run id ([977e90b](https://github.com/iandyh/shibuya/commit/977e90b0b8bb0d8a7795f3628a56d879688d8e8c))
* agent should also laod the config for object storage ([7c9f00c](https://github.com/iandyh/shibuya/commit/7c9f00c8788c522489265d4085ce093f02dbf7a1))
* better naming ([12a42de](https://github.com/iandyh/shibuya/commit/12a42de7e83c3e37f0e44a6fff923a5f59e48cfe))
* chart could not be generated due to tagging. Use gh cli directly instead of chart-releaser-action ([#111](https://github.com/iandyh/shibuya/issues/111)) ([8ab71bb](https://github.com/iandyh/shibuya/commit/8ab71bb47ce99c5c4d8e42976bcb277409f1354a))
* exclude scraper in engine details ([d3c302e](https://github.com/iandyh/shibuya/commit/d3c302ea420b55c1122ca01685603523ef8c247e))
* finish config factoring ([70a058f](https://github.com/iandyh/shibuya/commit/70a058facc008ba8c9d0a66be85f2f11f1a50a13))
* fix delete api ([cfc5126](https://github.com/iandyh/shibuya/commit/cfc5126d5643115d36f8b5a317b7cb5e5967d089))
* fix engine health check when the engines are down without notice ([a613a3f](https://github.com/iandyh/shibuya/commit/a613a3f4db5efba545c4833ed32893556588e8ba))
* fix go.sum ([999ee3d](https://github.com/iandyh/shibuya/commit/999ee3d08806b79e9722b738e75936bf4867be74))
* fix inventory could not remove collection data found in the ut ([2afaf1a](https://github.com/iandyh/shibuya/commit/2afaf1a6595c4c77fd557aa1cc3759a3b1beb54b))
* fix metric dashboard repo url ([#131](https://github.com/iandyh/shibuya/issues/131)) ([161c5e6](https://github.com/iandyh/shibuya/commit/161c5e64208dcc5637aaf899d1b81298ee40adc3))
* fix tests ([beff155](https://github.com/iandyh/shibuya/commit/beff1554b8f44b85b99bf7100d319c9704f167e1))
* fix the default spawn rate in locust ([3d259fb](https://github.com/iandyh/shibuya/commit/3d259fb965dd12b091570fef60d9c0059717fd16))
* fix typo in the Makefile ([82a5937](https://github.com/iandyh/shibuya/commit/82a5937618239be2903dc3e6ea64a681ff4f3f5b))
* fix typo in the path ([9cd74b7](https://github.com/iandyh/shibuya/commit/9cd74b7f2663591d3b8f9f01cb08e67d53ca4dd4))
* fix when terminate by plan ([4547715](https://github.com/iandyh/shibuya/commit/4547715d9026bd85512403a158aa9d9ce73606ac))
* give more resources to local engines ([59fc446](https://github.com/iandyh/shibuya/commit/59fc44627bb7c4964962d0c385a4282c5a602f50))
* ingress controller is renamed to coordinator and it is also built with github action ([286abbf](https://github.com/iandyh/shibuya/commit/286abbf2327630a078ee2c5afb9f55cda08bf489))
* local controller is a wrong binary ([8c729e4](https://github.com/iandyh/shibuya/commit/8c729e4e4bcb0a6b6e1f0af310d9c2b0653ca21f))
* make all should adapt to recent changes ([00d9d35](https://github.com/iandyh/shibuya/commit/00d9d35adf70fc854ae81943694642a42a635cec))
* make the sa token management more flexible ([476f545](https://github.com/iandyh/shibuya/commit/476f545c09add0a83ae4800bca3d615980082221))
* only build the image when it is a release ([d8fc0a1](https://github.com/iandyh/shibuya/commit/d8fc0a1496f591d6c9254460010b28e3187bf5d8))
* only set the imagepullsecret when there is a value ([7c4b0da](https://github.com/iandyh/shibuya/commit/7c4b0da651274c7bc5e743c3027385dc23da9ba4))
* prevent fork polutting the officical release registry ([77906e5](https://github.com/iandyh/shibuya/commit/77906e5140365321eb881d7c1edf2db1a94e1ae9))
* remove execess RBAC ([9c1e9d5](https://github.com/iandyh/shibuya/commit/9c1e9d53c076335683978bf4ba951b732dd987b1))
* remove logging ([#129](https://github.com/iandyh/shibuya/issues/129)) ([83f9353](https://github.com/iandyh/shibuya/commit/83f93539c5b579ce1448fbfa752e254e7c8a2d8e))
* remove unnecessary permissions for coordinator ([faaac80](https://github.com/iandyh/shibuya/commit/faaac80b98e85b1634d00cff4e6f5ebd8a2c3df2))
* replace gcr image and upgrade go to 1.23 ([317ec90](https://github.com/iandyh/shibuya/commit/317ec90639520c7466f66cc794cd086807451ca6))
* replace master with control ([5c9234e](https://github.com/iandyh/shibuya/commit/5c9234e6e89bd995c1050196277e49a797db5c88))
* request to metric gateway does not have path value ([a361ccf](https://github.com/iandyh/shibuya/commit/a361ccf17c96d4b341c65f33aa76db3c77b904e6))
* session store also need parseTime=true ([a4eeb43](https://github.com/iandyh/shibuya/commit/a4eeb43323b601f69fba518af0eff91f898aab62))
* should use release action from googleapi repo ([c38a4bb](https://github.com/iandyh/shibuya/commit/c38a4bb2aaeb172a4d1e44296715d950724f5008))
* upgrade go version to latest ([#133](https://github.com/iandyh/shibuya/issues/133)) ([a8dbe3b](https://github.com/iandyh/shibuya/commit/a8dbe3b45b49f4fef2db05b48f5cd60d7a295467))
* upgrade underscorejs to v1.13.7 ([8b8879a](https://github.com/iandyh/shibuya/commit/8b8879a407632a19769cbbf18afc8e04136bfbd4))
* use simple sleep for now ([bfccc51](https://github.com/iandyh/shibuya/commit/bfccc5125938892940fbf8a10f7d9b6d0faa4d1d))
* we should switch to histogram for aggregation ([018c0c9](https://github.com/iandyh/shibuya/commit/018c0c982cac4ff3e489121b735e87e149fd74b5))
* when no_auth is enabled, isadmin should be true ([b25ac09](https://github.com/iandyh/shibuya/commit/b25ac091132ecc93039d6c1bff58bafb7eafbaf9))
* when the ca_dir exists, do not exit ([7a2f2a8](https://github.com/iandyh/shibuya/commit/7a2f2a8ddb9bae5cad2792eb888b22c6d42634bb))
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
