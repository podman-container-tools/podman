//go:build !remote

// Package swagger defines the payloads used by the Podman API
//
//   - errors.go: declares the errors used in the API. By embedding errors.ErrorModel, more meaningful
//     comments can be provided for the developer documentation.
//   - models.go: declares the models used in API requests.
//   - responses.go: declares the responses used in the API responses.
//
// Notes:
//  1. As a developer of the Podman API, you are responsible for maintaining the associations between
//     these models and responses, and the handler code.
//  2. There are a number of warnings produces when compiling the swagger yaml file. This is expected.
//     Most are because embedded structs have been discovered but not used in the API declarations.
//  3. Response and model references that are exported (start with upper-case letter) imply that they
//     exist outside this package and should be found in the entities package.
package swagger
