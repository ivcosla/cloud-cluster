package cluster

import (
	"fmt"
	"net"
	"strconv"
	"sync/atomic"
	"time"

	consul "github.com/hashicorp/consul/api"
)

func (s *Server) SetupConsul() error {

	client, err := consul.NewClient(s.Config.ConsulConfig)
	if err != nil {
		return fmt.Errorf("Failed to setup consul: %v", err)
	}

	s.Agent = client.Agent()
	s.catalog = client.Catalog()

	if err := s.registerSerf(); err != nil {
		return err
	}

	go s.periodicHandler()

	return nil
}

func (s *Server) registerSerf() error {

	localNode := s.Serf.Memberlist().LocalNode()

	addr := localNode.Addr.String()
	port := int(localNode.Port)

	service := &consul.AgentServiceRegistration{
		ID:      s.Config.NodeName,
		Name:    s.Config.ServiceName,
		Tags:    []string{"serf"},
		Address: addr,
		Port:    port,
		Check: &consul.AgentServiceCheck{
			Interval: "5s",
			TCP:      fmt.Sprintf("%s:%d", s.Config.SerfConfig.MemberlistConfig.BindAddr, s.Config.SerfConfig.MemberlistConfig.BindPort),
		},
	}

	if err := s.Agent.ServiceRegister(service); err != nil {
		return fmt.Errorf("Failed to register service: %v", err)
	}

	return nil
}

func (s *Server) periodicHandler() {
	if err := s.bootstrap(); err != nil {
		panic(err)
	}

	for {
		select {
		case <-time.After(9 * time.Second):
			if err := s.bootstrap(); err != nil {
				s.logger.Printf("Bootstrap error: %v\n", err)
			}
		}
	}
}

func (s *Server) bootstrap() error {

	// Stop if we have already bootstraped everything
	bootstrapExpect := atomic.LoadInt32(&s.Config.BootstrapExpected)
	if bootstrapExpect == 0 {
		return nil
	}

	dcs, err := s.catalog.Datacenters()
	if err != nil {
		return fmt.Errorf("failed to get the datacenters: %v", err)
	}

	serverServices := []string{}
	localNode := s.Serf.Memberlist().LocalNode()
	for _, dc := range dcs {
		opts := &consul.QueryOptions{
			Datacenter: dc,
		}

		services, _, err := s.catalog.Service(s.Config.ServiceName, "serf", opts)
		if err != nil {
			return fmt.Errorf("failed to get the services: %v", err)
		}

		for _, service := range services {
			addr := service.ServiceAddress
			port := service.ServicePort

			if addr == "" {
				addr = service.Address
			}

			if localNode.Addr.String() == addr && int(localNode.Port) == port {
				continue
			}

			serviceAddr := net.JoinHostPort(addr, strconv.FormatInt(int64(port), 10))
			serverServices = append(serverServices, serviceAddr)
		}
	}

	if _, err = s.Serf.Join(serverServices, true); err != nil {
		return fmt.Errorf("failed to join: %v", err)
	}

	return nil
}
