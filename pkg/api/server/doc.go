//go:build !remote

// Package server supports a RESTful API for the Libpod library
//
// This documentation describes the Podman v2.x+ RESTful API. It consists of a Docker-compatible
// API and a Libpod API providing support for Podman’s unique features such as pods.
//
// To start the service and keep it running for 5,000 seconds (-t 0 runs forever):
//
//	podman system service -t 5000 &
//
// You can then use cURL on the socket using requests documented below.
//
// NOTE: if you install the package podman-docker, it will create a symbolic
// link for /run/docker.sock to /run/podman/podman.sock
//
// NOTE: Some fields in the API response JSON are encoded as omitempty, which means that
// if said field has a zero value, they will not be encoded in the API response. This
// is a feature to help reduce the size of the JSON responses returned via the API.
//
// NOTE: Due to the limitations of [go-swagger](https://github.com/go-swagger/go-swagger),
// some field values that have a complex type show up as null in the docs as well as in the
// API responses. This is because the zero value for the field type is null. The field
// description in the docs will state what type the field is expected to be for such cases.
//
// See podman-system-service(1) for more information.
//
//	Quick Examples:
//
//	 'podman info'
//
//	    curl --unix-socket /run/podman/podman.sock http://d/v6.0.0/libpod/info
//
//	 'podman pull quay.io/containers/podman'
//
//	    curl -XPOST --unix-socket /run/podman/podman.sock -v 'http://d/v6.0.0/images/create?fromImage=quay.io%2Fcontainers%2Fpodman'
//
//	 'podman list images'
//
//	    curl --unix-socket /run/podman/podman.sock -v 'http://d/v6.0.0/libpod/images/json' | jq
//
// Terms Of Service:
//
// https://github.com/containers/podman/blob/913caaa9b1de2b63692c9bae15120208194c9eb3/LICENSE
//
//	Schemes: http, https
//	Host: podman.io
//	BasePath: /
//	Version: 6.0.0
//	License: Apache-2.0 https://opensource.org/licenses/Apache-2.0
//	Contact: Podman <podman@lists.podman.io> https://podman.io/community
//
//	InfoExtensions:
//	x-logo:
//	  - url: https://raw.githubusercontent.com/podman-container-tools/podman/main/logo/podman-logo.png
//	  - altText: "Podman logo"
//
//	Produces:
//	- application/json
//	- application/octet-stream
//	- text/plain
//
//	Consumes:
//	- application/json
//	- application/x-tar
//
// swagger:meta
package server
