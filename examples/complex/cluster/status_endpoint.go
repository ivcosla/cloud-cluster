package cluster

type Status struct {
	srv *Server
}

func (s *Status) Peers(args interface{}, reply *[]string) error {
	if done, err := s.srv.forward("Status.Peers", args, reply); done {
		return err
	}

	future := s.srv.cluster.Raft.GetConfiguration()
	if err := future.Error(); err != nil {
		return err
	}

	for _, server := range future.Configuration().Servers {
		*reply = append(*reply, string(server.Address))
	}
	return nil
}
