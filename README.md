
[![Report](https://goreportcard.com/badge/github.com/loupzeur/goinsee)](https://goreportcard.com/report/github.com/loupzeur/goinsee)
[![Report](https://img.shields.io/badge/license-MIT-brightgreen.svg)](https://opensource.org/licenses/MIT)
[![doc](https://camo.githubusercontent.com/d1a67a692a0fa15f86748f98a790a28b2086e50ee6cc85015010745183b26eed/68747470733a2f2f696d672e736869656c64732e696f2f62616467652f676f2e6465762d7265666572656e63652d626c75653f6c6f676f3d676f266c6f676f436f6c6f723d7768697465)](https://pkg.go.dev/github.com/loupzeur/goinsee)
![build workflow](https://github.com/loupzeur/goinsee/actions/workflows/go.yml/badge.svg)

# Check Siren validity

## Overview

Simply provide the key and secret of the API
```
i := NewInsee(os.Getenv("insee_key"), os.Getenv("insee_secret"))
i.SirenExist("443061841")
```

If you are on a server that will last more than 7 days up, you will require to refresh the token which only valid for 7 days, you can use the one that will refresh itself automatically
```
i := NewInseeRefreshed(os.Getenv("insee_key"), os.Getenv("insee_secret"))
i.SirenExist("443061841")
```

## May provide others options from Insee database API

Return the value of the call from sirene API
```
i := NewInsee(os.Getenv("insee_key"), os.Getenv("insee_secret"))
i.GetSiren("443061841")
```
