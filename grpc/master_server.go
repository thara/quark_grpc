package grpc

import (
	"context"

	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"quark"
	"quark/masterserver"
	"quark/proto"
	"quark/proto/primitive"
)

const GameServerIDMetadataKey = "quark-gameserver-id"

type masterServer struct {
	proto.UnimplementedMasterServerServer

	fleet *masterserver.Fleet
}

func NewMasterServer(fleet *masterserver.Fleet) proto.MasterServerServer {
	return &masterServer{fleet: fleet}
}

func (s *masterServer) RegisterGameServer(req *proto.RegisterGameServerRequest, stream proto.MasterServer_RegisterGameServerServer) error {
	if req.NewGameServer == nil {
		return status.Errorf(codes.InvalidArgument, "NewGameServer is required")
	}
	gs := req.NewGameServer
	if len(gs.Address) == 0 {
		return status.Errorf(codes.InvalidArgument, "NewGameServer.Address must not be empty")
	}
	if len(gs.Port) == 0 {
		return status.Errorf(codes.InvalidArgument, "NewGameServer.Port must not be empty")
	}

	addr := masterserver.GameServerAddr{Addr: gs.Address, Port: gs.Port}
	gameServerID := s.fleet.RegisterGameServer(masterserver.GameServerAddr{Addr: gs.Address, Port: gs.Port}, 5)

	err := stream.Send(&proto.MasterServerMessage{
		Message: &proto.MasterServerMessage_Registered{
			Registered: &proto.MasterServerMessage_GameServerRegistered{
				GameServerID: string(gameServerID),
			},
		},
	})
	if err != nil {
		return err
	}

	c := make(chan masterserver.RoomAllocatedEvent)
	s.fleet.AddRoomAllocationListener(c)
	defer func() {
		s.fleet.RemoveRoomAllocationListener(c)
		close(c)
	}()

	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case ev := <-c:
			if addr == ev.GameServer {
				m := &proto.MasterServerMessage{
					Message: &proto.MasterServerMessage_Allocation{
						Allocation: &proto.MasterServerMessage_RoomAllocation{
							Room: &primitive.Room{
								RoomID:   ev.Room.RoomID.Uint64(),
								RoomName: ev.Room.RoomName,
							},
						},
					},
				}
				err := stream.Send(m)
				if err != nil {
					return err
				}
			}
		}
	}
}

func (s *masterServer) Update(stream proto.MasterServer_UpdateServer) error {
	gameServerID, ok := getGameServerID(stream.Context())
	if !ok {
		return status.Errorf(codes.PermissionDenied, "game server ID is required in metadata")
	}
	if !s.fleet.IsRegisteredGameServer(gameServerID) {
		return status.Errorf(codes.PermissionDenied, "invalid game server ID")
	}

	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		default:
			m, err := stream.Recv()
			if err != nil {
				return errors.WithStack(err)
			}
			for _, r := range m.UpdateRoomState {
				newStatus := masterserver.RoomStatus{RoomID: quark.RoomID(r.Room.RoomID), RoomName: r.Room.RoomName, ActorCount: uint(r.ActorCount)}
				err := s.fleet.UpdateRoomStatus(newStatus)
				if err != nil {
					return errors.WithStack(err)
				}
			}
		}
	}
}

func getGameServerID(ctx context.Context) (masterserver.GameServerID, bool) {
	m, ok := metadata.FromIncomingContext(ctx)
	if !ok || len(m[GameServerIDMetadataKey]) == 0 {
		return "", false
	}
	s := m[GameServerIDMetadataKey][0]
	return masterserver.GameServerID(s), true
}
