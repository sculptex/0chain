package conductrpc

import (
	"net/rpc"
)

// Client of the conductor RPC server.
type Client struct {
	address string
	client  *rpc.Client
}

// NewClient creates new client will be interacting
// with server with given address.
func NewClient(address string) (c *Client, err error) {
	if address, err = Host(address); err != nil {
		return
	}
	c = new(Client)
	if c.client, err = rpc.Dial("tcp", address); err != nil {
		return nil, err
	}
	c.address = address
	return
}

func (c *Client) dial() (err error) {
	c.client, err = rpc.Dial("tcp", c.address)
	return
}

// Address of RPC server.
func (c *Client) Address() string {
	return c.address
}

//
// miner SC RPC
//

func (c *Client) Phase(phase *PhaseEvent) (err error) {
	err = c.client.Call("Server.Phase", phase, &struct{}{})
	if err == rpc.ErrShutdown {
		if err = c.dial(); err != nil {
			return
		}
		err = c.client.Call("Server.Phase", phase, &struct{}{})
	}
	return
}

// ViewChange notification.
func (c *Client) ViewChange(viewChange *ViewChangeEvent) (err error) {
	err = c.client.Call("Server.ViewChange", viewChange, &struct{}{})
	if err == rpc.ErrShutdown {
		if err = c.dial(); err != nil {
			return
		}
		err = c.client.Call("Server.ViewChange", viewChange, &struct{}{})
	}
	return
}

// AddMiner notification.
func (c *Client) AddMiner(add *AddMinerEvent) (err error) {
	err = c.client.Call("Server.AddMiner", add, &struct{}{})
	if err == rpc.ErrShutdown {
		if err = c.dial(); err != nil {
			return
		}
		err = c.client.Call("Server.AddMiner", add, &struct{}{})
	}
	return
}

// AddSharder notification.
func (c *Client) AddSharder(add *AddSharderEvent) (err error) {
	err = c.client.Call("Server.AddSharder", add, &struct{}{})
	if err == rpc.ErrShutdown {
		if err = c.dial(); err != nil {
			return
		}
		err = c.client.Call("Server.AddSharder", add, &struct{}{})
	}
	return
}

// NodeReady notification.
func (c *Client) NodeReady(nodeID NodeID) (join bool, err error) {
	err = c.client.Call("Server.NodeReady", nodeID, &join)
	if err == rpc.ErrShutdown {
		if err = c.dial(); err != nil {
			return
		}
		err = c.client.Call("Server.NodeReady", nodeID, &join)
	}
	return
}
