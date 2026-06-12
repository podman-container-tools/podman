package autoupdate

import (
	"context"
	"net/http"
	"strconv"

	imageTypes "go.podman.io/image/v5/types"
	"go.podman.io/podman/v6/pkg/api/handlers"
	"go.podman.io/podman/v6/pkg/auth"
	"go.podman.io/podman/v6/pkg/bindings"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/errorhandling"
)

func AutoUpdate(ctx context.Context, options *AutoUpdateOptions) ([]*entities.AutoUpdateReport, []error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, []error{err}
	}
	if options == nil {
		options = new(AutoUpdateOptions)
	}

	params, err := options.ToParams()
	if err != nil {
		return nil, []error{err}
	}
	// InsecureSkipTLSVerify is special.  We need to delete the param added by
	// ToParams() and change the key and flip the bool
	if options.InsecureSkipTLSVerify != nil {
		params.Del("SkipTLSVerify")
		params.Set("tlsVerify", strconv.FormatBool(!options.GetInsecureSkipTLSVerify()))
	}

	header, err := auth.MakeXRegistryAuthHeader(&imageTypes.SystemContext{AuthFilePath: options.GetAuthfile()}, "", "")
	if err != nil {
		return nil, []error{err}
	}

	response, err := conn.DoRequest(ctx, nil, http.MethodPost, "/autoupdate", params, header)
	if err != nil {
		return nil, []error{err}
	}
	defer response.Body.Close()

	var reports handlers.LibpodAutoUpdateReports

	if err := response.Process(&reports); err != nil {
		return nil, []error{err}
	}

	return reports.Reports, errorhandling.StringsToErrors(reports.Errors)
}
