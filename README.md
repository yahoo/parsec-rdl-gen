# parsec-rdl-gen [![Build Status](https://app.travis-ci.com/yahoo/parsec-rdl-gen.svg?branch=master)](https://app.travis-ci.com/yahoo/parsec-rdl-gen)

Parsec Ardielle (RDL) External Generators

* parsec-java-model - generator for generating Parsec Java models
* parsec-java-server - generator for generating Parsec Java server
* parsec-java-client - generator for generating Parsec Java client for target web service
* parsec-swagger - generator for generating Swagger JSON schemas

## Usage

These generators are designed to co-work with [ardielle-tools](https://github.com/ardielle/ardielle-tools) but can also be used independently.  They are executable binaries and takes JSON representation of Ardielle schemas from StdIn.  

Sample usage for co-working with [ardielle-tools](https://github.com/ardielle/ardielle-tools):

    rdl generate [options] <parsec-java-model | parsec-java-server | parsec-java-client | parsec-swagger> <schema.rdl>

Please refer to [ardielle-tools](https://github.com/ardielle/ardielle-tools) for more information.

## How to build

Please follow https://golang.org/doc/install to download and install the GO. You also need to set the GOPATH environment, the source code to checkout and build would belong this GOPATH setting, for instance, I set the GOPATH to /Users/guang001/Documents/workspace/go, then I execute the command: 
```
go get github.com/yahoo/parsec-rdl-gen/...
```

Then GO will checkout the source code to $GOPATH/src/github.com/yahoo/parsec-rdl-gen/, this should be /Users/guang001/Documents/workspace/go/src/github.com/yahoo/parsec-rdl-gen/ in my case, and 'go get' command will build the binary after the fetch code, the binary would be put in $GOPATH/bin/ path. The detail you could reference [How to Write Go Code](https://golang.org/doc/code.html).

So, to build the parsec-rdl-gen, you only need execute: 'go get github.com/yahoo/parsec-rdl-gen/...' if you are ready the GO enviroment. If you need switch the git branch, you could:
```
cd /Users/guang001/Documents/workspace/go/src/github.com/yahoo/parsec-rdl-gen/
git checkout -b dev
```

or change to your fork REPO:

```
cd /Users/guang001/Documents/workspace/go/src/github.com/yahoo/parsec-rdl-gen/
git remote set-url origin git@git.corp.yahoo.com:guang001/apex.git
```

Note: The 'go get' command would not refetch(checkout) the code if target directory already exist.

## License

Copyright 2016 Yahoo Inc.
Licensed under the terms of the Apache license. Please see LICENSE.md file distributed with this work for terms.
