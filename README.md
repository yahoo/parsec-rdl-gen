# parsec-rdl-gen

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

## License

Copyright 2016 Yahoo Inc.
Licensed under the terms of the Apache license. Please see LICENSE.md file distributed with this work for terms.
