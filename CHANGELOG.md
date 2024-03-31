# Changelog

## [v0.11.4](https://github.com/Songmu/ecschedule/compare/v0.11.3...v0.11.4) - 2024-03-31
- Fix retrieval of AWS::Events::Rule SearchResources results by @kenkaton in https://github.com/Songmu/ecschedule/pull/97
- build(deps): bump google.golang.org/protobuf from 1.31.0 to 1.33.0 by @dependabot in https://github.com/Songmu/ecschedule/pull/96
- Go 1.22 and update deps by @Songmu in https://github.com/Songmu/ecschedule/pull/99

## [v0.11.3](https://github.com/Songmu/ecschedule/compare/v0.11.2...v0.11.3) - 2023-12-31
- build(deps): bump golang.org/x/crypto from 0.15.0 to 0.17.0 by @dependabot in https://github.com/Songmu/ecschedule/pull/94

## [v0.11.2](https://github.com/Songmu/ecschedule/compare/v0.11.1...v0.11.2) - 2023-11-10
- Care remote PropagateTags default value by @lamanotrama in https://github.com/Songmu/ecschedule/pull/89
- build(deps): bump google.golang.org/grpc from 1.58.2 to 1.58.3 by @dependabot in https://github.com/Songmu/ecschedule/pull/88
- update deps by @Songmu in https://github.com/Songmu/ecschedule/pull/91

## [v0.11.1](https://github.com/Songmu/ecschedule/compare/v0.11.0...v0.11.1) - 2023-10-12
- docs: add the installation guides with aqua by @suzuki-shunsuke in https://github.com/Songmu/ecschedule/pull/85
- build(deps): bump golang.org/x/net from 0.16.0 to 0.17.0 by @dependabot in https://github.com/Songmu/ecschedule/pull/87

## [v0.11.0](https://github.com/Songmu/ecschedule/compare/v0.10.3...v0.11.0) - 2023-10-08
- Upgrade AWS SDK for Go from V1 to V2 by @snaka in https://github.com/Songmu/ecschedule/pull/78
- Go 1.21 and update deps by @Songmu in https://github.com/Songmu/ecschedule/pull/83

## [v0.10.3](https://github.com/Songmu/ecschedule/compare/v0.10.2...v0.10.3) - 2023-10-06
- update to tfstate-lookup v1.1.4 by @fujiwara in https://github.com/Songmu/ecschedule/pull/79

## [v0.10.2](https://github.com/Songmu/ecschedule/compare/v0.10.1...v0.10.2) - 2023-09-01
- Align remote and conf override members by @lamanotrama in https://github.com/Songmu/ecschedule/pull/76

## [v0.10.1](https://github.com/Songmu/ecschedule/compare/v0.10.0...v0.10.1) - 2023-08-25
- docs: Add description of `-prune` option by @snaka in https://github.com/Songmu/ecschedule/pull/73
- fix: `-conf` omitted caused panic by referencing a nil pointer by @snaka in https://github.com/Songmu/ecschedule/pull/74

## [v0.10.0](https://github.com/Songmu/ecschedule/compare/v0.9.1...v0.10.0) - 2023-08-20
- update deps by @Songmu in https://github.com/Songmu/ecschedule/pull/68
- fix typo by @snaka in https://github.com/Songmu/ecschedule/pull/72
- feat: Adding a `-prune` option to remove Orphaned Rules. by @snaka in https://github.com/Songmu/ecschedule/pull/71

## [v0.9.1](https://github.com/Songmu/ecschedule/compare/v0.9.0...v0.9.1) - 2023-02-22
- chore: improve install guide by @paprika-mah in https://github.com/Songmu/ecschedule/pull/64
- build(deps): bump golang.org/x/net from 0.2.0 to 0.7.0 by @dependabot in https://github.com/Songmu/ecschedule/pull/65

## [v0.9.0](https://github.com/Songmu/ecschedule/compare/v0.8.1...v0.9.0) - 2023-02-08
- Support resource overrides by @lamanotrama in https://github.com/Songmu/ecschedule/pull/62

## [v0.8.1](https://github.com/Songmu/ecschedule/compare/v0.8.0...v0.8.1) - 2023-01-17
- fix the error that occurs when the input is nil by @sinsoku in https://github.com/Songmu/ecschedule/pull/60

## [v0.8.0](https://github.com/Songmu/ecschedule/compare/v0.7.2...v0.8.0) - 2023-01-16
- support json jsonnet by @mrymam in https://github.com/Songmu/ecschedule/pull/58

## [v0.7.2](https://github.com/Songmu/ecschedule/compare/v0.7.1...v0.7.2) - 2022-12-23
- Update version description "v0.3.1" to "v0.7.1" in  README by @TanisukeGoro in https://github.com/Songmu/ecschedule/pull/53
- fix: PropagateTags not applied by @sinsi404 in https://github.com/Songmu/ecschedule/pull/55

## [v0.7.1](https://github.com/Songmu/ecschedule/compare/v0.7.0...v0.7.1) - 2022-11-21
- fix a shell command to expand variables by @hoyo in https://github.com/Songmu/ecschedule/pull/51

## [v0.7.0](https://github.com/Songmu/ecschedule/compare/v0.6.3...v0.7.0) - 2022-11-20
- add install.sh by @Songmu in https://github.com/Songmu/ecschedule/pull/45
- add SHA256SUMS to artifacts by @Songmu in https://github.com/Songmu/ecschedule/pull/46
- introduce tagpr by @Songmu in https://github.com/Songmu/ecschedule/pull/47
- update deps except for go-ieproxy by @Songmu in https://github.com/Songmu/ecschedule/pull/49
- update action.yml to use installer by @Songmu in https://github.com/Songmu/ecschedule/pull/50

## [v0.6.3](https://github.com/Songmu/ecschedule/compare/v0.6.2...v0.6.3) (2022-07-12)

* update deps except for go-ieproxy [#44](https://github.com/Songmu/ecschedule/pull/44) ([Songmu](https://github.com/Songmu))

## [v0.6.2](https://github.com/Songmu/ecschedule/compare/v0.6.1...v0.6.2) (2022-07-12)

* fix: allow undefined for PropagateTags [#42](https://github.com/Songmu/ecschedule/pull/42) ([gotoeveryone](https://github.com/gotoeveryone))

## [v0.6.1](https://github.com/Songmu/ecschedule/compare/v0.6.0...v0.6.1) (2022-06-15)

* [bugfix] Fixed the diff of propagateTags to appear correctly. [#41](https://github.com/Songmu/ecschedule/pull/41) ([cohalz](https://github.com/cohalz))
* add pitfall warnings to README [#40](https://github.com/Songmu/ecschedule/pull/40) ([Songmu](https://github.com/Songmu))

## [v0.6.0](https://github.com/Songmu/ecschedule/compare/v0.5.2...v0.6.0) (2022-06-02)

* feat: add propagateTags option [#39](https://github.com/Songmu/ecschedule/pull/39) ([cohalz](https://github.com/cohalz))

## [v0.5.2](https://github.com/Songmu/ecschedule/compare/v0.5.1...v0.5.2) (2022-05-25)

* fix: can execute release workflow to fix darwin builds [#38](https://github.com/Songmu/ecschedule/pull/38) ([gotoeveryone](https://github.com/gotoeveryone))
* update github actions [#36](https://github.com/Songmu/ecschedule/pull/36) ([Songmu](https://github.com/Songmu))

## [v0.5.1](https://github.com/Songmu/ecschedule/compare/v0.5.0...v0.5.1) (2022-05-06)

* Go 1.18 and update deps [#35](https://github.com/Songmu/ecschedule/pull/35) ([Songmu](https://github.com/Songmu))
* introduce reviewdog and drop golint [#34](https://github.com/Songmu/ecschedule/pull/34) ([Songmu](https://github.com/Songmu))
* fix: golint error [#33](https://github.com/Songmu/ecschedule/pull/33) ([gotoeveryone](https://github.com/gotoeveryone))

## [v0.5.0](https://github.com/Songmu/ecschedule/compare/v0.4.0...v0.5.0) (2022-05-06)

* enable tfstate plugin [#32](https://github.com/Songmu/ecschedule/pull/32) ([gotoeveryone](https://github.com/gotoeveryone))

## [v0.4.0](https://github.com/Songmu/ecschedule/compare/v0.3.2...v0.4.0) (2022-01-20)

* support deadletter config [#30](https://github.com/Songmu/ecschedule/pull/30) ([MikiWaraMiki](https://github.com/MikiWaraMiki))

## [v0.3.2](https://github.com/Songmu/ecschedule/compare/v0.3.1...v0.3.2) (2021-12-28)

* Go 1.17 and follow it in toolchains [#29](https://github.com/Songmu/ecschedule/pull/29) ([Songmu](https://github.com/Songmu))
* added task definition validation. [#28](https://github.com/Songmu/ecschedule/pull/28) ([reiki4040](https://github.com/reiki4040))
* Add procedure of execute `run` subcommand to README [#27](https://github.com/Songmu/ecschedule/pull/27) ([gotoeveryone](https://github.com/gotoeveryone))
* Add "rules:" to the sample configuration [#25](https://github.com/Songmu/ecschedule/pull/25) ([yuu26jp](https://github.com/yuu26jp))
* Add action.yml for GitHub Actions [#24](https://github.com/Songmu/ecschedule/pull/24) ([mokichi](https://github.com/mokichi))

## [v0.3.1](https://github.com/Songmu/ecschedule/compare/v0.3.0...v0.3.1) (2021-02-06)

* Fixed yaml from scheduledExpression to scheduleExpression [#23](https://github.com/Songmu/ecschedule/pull/23) ([yutachaos](https://github.com/yutachaos))
* fix the variable Run Task Input when running rule. [#22](https://github.com/Songmu/ecschedule/pull/22) ([laughk](https://github.com/laughk))
* enable shared config state [#21](https://github.com/Songmu/ecschedule/pull/21) ([tughril](https://github.com/tughril))

## [v0.3.0](https://github.com/Songmu/ecschedule/compare/v0.2.0...v0.3.0) (2020-11-22)

* Add ecs parameters to rule [#20](https://github.com/Songmu/ecschedule/pull/20) ([fujiwara](https://github.com/fujiwara))

## [v0.2.0](https://github.com/Songmu/ecschedule/compare/v0.1.2...v0.2.0) (2020-11-15)

* update deps [#19](https://github.com/Songmu/ecschedule/pull/19) ([Songmu](https://github.com/Songmu))
* rename to ecschedule from ecsched [#18](https://github.com/Songmu/ecschedule/pull/18) ([Songmu](https://github.com/Songmu))

## [v0.1.2](https://github.com/Songmu/ecschedule/compare/v0.1.1...v0.1.2) (2020-11-09)

* fix error handling of ListRules [#17](https://github.com/Songmu/ecschedule/pull/17) ([Songmu](https://github.com/Songmu))

## [v0.1.1](https://github.com/Songmu/ecschedule/compare/v0.1.0...v0.1.1) (2020-11-04)

* implement diff -all option [#16](https://github.com/Songmu/ecschedule/pull/16) ([Songmu](https://github.com/Songmu))
* implement apply -all option [#15](https://github.com/Songmu/ecschedule/pull/15) ([Songmu](https://github.com/Songmu))

## [v0.1.0](https://github.com/Songmu/ecschedule/compare/v0.0.2...v0.1.0) (2020-11-04)

* update documents [#14](https://github.com/Songmu/ecschedule/pull/14) ([Songmu](https://github.com/Songmu))
* define type runnerImpl and refacter [#13](https://github.com/Songmu/ecschedule/pull/13) ([Songmu](https://github.com/Songmu))
* display diff before applying [#12](https://github.com/Songmu/ecschedule/pull/12) ([Songmu](https://github.com/Songmu))

## [v0.0.2](https://github.com/Songmu/ecschedule/compare/v0.0.1...v0.0.2) (2020-11-02)

* update deps [#11](https://github.com/Songmu/ecschedule/pull/11) ([Songmu](https://github.com/Songmu))
* take all rules on dump with caring nextToken [#10](https://github.com/Songmu/ecschedule/pull/10) ([Songmu](https://github.com/Songmu))
* add diff subcommand [#9](https://github.com/Songmu/ecschedule/pull/9) ([Songmu](https://github.com/Songmu))
* check mustEnv lazily [#8](https://github.com/Songmu/ecschedule/pull/8) ([Songmu](https://github.com/Songmu))
* care empty rule.Description [#7](https://github.com/Songmu/ecschedule/pull/7) ([Songmu](https://github.com/Songmu))
* update deps [#6](https://github.com/Songmu/ecschedule/pull/6) ([Songmu](https://github.com/Songmu))
* add json tags for containerOverrides [#5](https://github.com/Songmu/ecschedule/pull/5) ([Songmu](https://github.com/Songmu))
* introduce goccy/go-yaml [#4](https://github.com/Songmu/ecschedule/pull/4) ([Songmu](https://github.com/Songmu))
* adjust yamls for GitHub Actions [#3](https://github.com/Songmu/ecschedule/pull/3) ([Songmu](https://github.com/Songmu))

## [v0.0.1](https://github.com/Songmu/ecschedule/compare/1ca37db7d7e6...v0.0.1) (2019-10-26)

* add release.yaml [#2](https://github.com/Songmu/ecschedule/pull/2) ([Songmu](https://github.com/Songmu))
* introduce github action [#1](https://github.com/Songmu/ecschedule/pull/1) ([Songmu](https://github.com/Songmu))
