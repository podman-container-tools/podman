package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/pkg/api/grpcpb"
	"go.podman.io/podman/v6/pkg/bindings"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	reflectionv1 "google.golang.org/grpc/reflection/grpc_reflection_v1"
)

var (
	noopDescription = `Call the no-op GRPC endpoint.`
	noopCmd         = &cobra.Command{
		Use:               "noop [arg]",
		Args:              cobra.MaximumNArgs(1),
		Short:             "Call the no-op GRPC endpoint",
		Long:              noopDescription,
		RunE:              noop,
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman-testing noop`,
	}
	lsDescription = `List GRPC endpoints.`
	lsCmd         = &cobra.Command{
		Use:               "ls [arg]",
		Args:              cobra.MaximumNArgs(1),
		Short:             "Call an RPC endpoint",
		Long:              lsDescription,
		RunE:              ls,
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman-testing ls`,
	}
)

func init() {
	mainCmd.AddCommand(noopCmd)
	mainCmd.AddCommand(lsCmd)
}

func ls(_ *cobra.Command, args []string) error {
	if podmanConfig.EngineMode != entities.TunnelMode {
		return errors.New("only available in remote mode")
	}
	ctx, grpcClient, err := getGrpcClient()
	if err != nil {
		return fmt.Errorf("setting up grpc client for podman service: %w", err)
	}
	reflectionClient := reflectionv1.NewServerReflectionClient(grpcClient)
	info, err := reflectionClient.ServerReflectionInfo(ctx)
	if err != nil {
		return fmt.Errorf("reflection grpc service: %w", err)
	}
	ls := ""
	if len(args) > 1 {
		ls = args[1]
	}
	err = info.Send(&reflectionv1.ServerReflectionRequest{
		MessageRequest: &reflectionv1.ServerReflectionRequest_ListServices{
			ListServices: ls,
		},
	})
	if err != nil {
		return fmt.Errorf("reflection grpc service: %w", err)
	}
	err = info.CloseSend()
	if err != nil {
		return fmt.Errorf("reflection grpc service: %w", err)
	}
	var response reflectionv1.ServerReflectionResponse
	err = info.RecvMsg(&response)
	for err == nil {
		var out []byte
		out, err = json.Marshal(response.GetListServicesResponse())
		if err != nil {
			return fmt.Errorf("encoding response from grpc service: %w", err)
		}
		fmt.Println(string(out))
		response.Reset()
		err = info.RecvMsg(&response)
	}
	if !errors.Is(err, io.EOF) {
		return fmt.Errorf("unexpected grpc protocol error: %w", err)
	}
	return nil
}

func noop(_ *cobra.Command, args []string) error {
	var out []byte
	switch podmanConfig.EngineMode {
	case entities.TunnelMode:
		ctx, grpcClient, err := getGrpcClient()
		if err != nil {
			return fmt.Errorf("setting up grpc client for podman service: %w", err)
		}
		noopClient := grpcpb.NewNoopClient(grpcClient)
		var request grpcpb.NoopRequest
		if encoded := strings.Join(args, ""); len(encoded) > 0 {
			if err := json.Unmarshal([]byte(encoded), &request); err != nil {
				return fmt.Errorf("parsing client request contents for noop grpc service: %w", err)
			}
		}
		response, err := noopClient.Noop(ctx, &request)
		if err != nil {
			return fmt.Errorf("noop grpc service: %w", err)
		}
		out, err = json.Marshal(response)
		if err != nil {
			return fmt.Errorf("encoding response from grpc service: %w", err)
		}
	default:
		return errors.New("only available in remote mode")
	}
	fmt.Println(string(out))
	return nil
}

func getGrpcClient() (context.Context, *grpc.ClientConn, error) {
	ctx, err := bindings.NewConnection(mainContext, podmanConfig.URI)
	if err != nil {
		return nil, nil, fmt.Errorf("connecting to podman service: %w", err)
	}
	client, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("obtaining client handle for podman service: %w", err)
	}
	onlyPodmanSystemServiceDialer := grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return client.GetDialer(ctx) })
	withoutEncryption := grpc.WithTransportCredentials(insecure.NewCredentials())
	grpcClient, err := grpc.NewClient(podmanConfig.URI, onlyPodmanSystemServiceDialer, withoutEncryption)
	if err != nil {
		return nil, nil, fmt.Errorf("setting up grpc client for podman service: %w", err)
	}
	return ctx, grpcClient, err
}
