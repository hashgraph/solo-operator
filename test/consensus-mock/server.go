// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"

	"google.golang.org/grpc"

	services "github.com/hiero-ledger/hiero-sdk-go/v2/proto/services"
)

// networkServer answers just enough of NetworkService for beacon's
// reachability preflight (internal/beacon/client/client.go:probeHAPI issues a
// NetworkVersionQuery before every command).
type networkServer struct {
	services.UnimplementedNetworkServiceServer
}

func (networkServer) GetVersionInfo(context.Context, *services.Query) (*services.Response, error) {
	return &services.Response{
		Response: &services.Response_NetworkGetVersionInfo{
			NetworkGetVersionInfo: &services.NetworkGetVersionInfoResponse{
				Header:                &services.ResponseHeader{NodeTransactionPrecheckCode: services.ResponseCodeEnum_OK},
				HapiProtoVersion:      &services.SemanticVersion{Major: 0, Minor: 60, Patch: 0},
				HederaServicesVersion: &services.SemanticVersion{Major: 0, Minor: 60, Patch: 0},
			},
		},
	}, nil
}

// cryptoServer answers just enough of CryptoService for
// internal/beacon/client/submit.go:Submit's post-execute receipt poll.
// consensus-mock has no real consensus to poll for, so every receipt is an
// immediate SUCCESS.
type cryptoServer struct {
	services.UnimplementedCryptoServiceServer
}

func (cryptoServer) GetTransactionReceipts(context.Context, *services.Query) (*services.Response, error) {
	return &services.Response{
		Response: &services.Response_TransactionGetReceipt{
			TransactionGetReceipt: &services.TransactionGetReceiptResponse{
				Header:  &services.ResponseHeader{NodeTransactionPrecheckCode: services.ResponseCodeEnum_OK},
				Receipt: &services.TransactionReceipt{Status: services.ResponseCodeEnum_SUCCESS},
			},
		},
	}, nil
}

func newGRPCServer(fz *freezeServer) *grpc.Server {
	s := grpc.NewServer()
	services.RegisterFreezeServiceServer(s, fz)
	services.RegisterNetworkServiceServer(s, networkServer{})
	services.RegisterCryptoServiceServer(s, cryptoServer{})
	return s
}
