package load

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto/ed25519"
	cmtnet "github.com/tendermint/tendermint/libs/net"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/conn"
	"github.com/tendermint/tendermint/version"
)

var defaultProtocolVersion = p2p.NewProtocolVersion(
	version.P2PProtocol,
	version.BlockProtocol,
	0,
)

var sendTime time.Time

func TestTransportBench(t *testing.T) {
	cfg := config.DefaultP2PConfig()
	mcfg := conn.DefaultMConnConfig()
	mcfg.SendRate = 1000000
	mcfg.RecvRate = 1000000

	reactor1 := NewMockReactor(defaultTestChannels)
	node1, err := newnode(*cfg, mcfg, reactor1)
	require.NoError(t, err)

	reactor2 := NewMockReactor(defaultTestChannels)
	node2, err := newnode(*cfg, mcfg, reactor2)
	require.NoError(t, err)

	err = node1.start()
	require.NoError(t, err)
	defer node1.stop()

	err = node2.start()
	require.NoError(t, err)
	defer node2.stop()
	time.Sleep(1 * time.Second) // wait for the nodes to startup

	err = node2.sw.DialPeerWithAddress(node1.addr)
	require.NoError(t, err)
	time.Sleep(1 * time.Second) // wait for the nodes to connect

	// reactor1.FillChannel(SecondChannel, 1000, 10000)
	// reactor1.FillChannel(ThirdChannel, 1000, 10000)
	time.Sleep(100 * time.Millisecond)     // wait for the messages to start sending
	reactor1.SendBytes(FirstChannel, 2000) // send a messasge on the first channel and see how long it takes to receive
	sendTime = time.Now()
	time.Sleep(5 * time.Second) // wait for the messages to be send
	require.Greater(t, len(reactor2.Traces), 0)
	// VizBandwidth("test.png", reactor2.Traces)
	// VizTotalBandwidth("test2.png", reactor2.Traces)
	for _, m := range reactor2.Traces {
		if m.Channel == FirstChannel {
			fmt.Println(m.ReceiveTime.Sub(sendTime).Milliseconds())
		}
	}
}

/*
The next steps are to compare the rates of a prioritized channel that is also filled.

How to compare rate? We can compare the traces of the sending and receiving, that te

*/

type node struct {
	key ed25519.PrivKey
	id  p2p.ID
	// cfg    peerConfig
	p2pCfg config.P2PConfig
	addr   *p2p.NetAddress
	sw     *p2p.Switch
	mt     *p2p.MultiplexTransport
}

// newnode creates a new local peer with a random key.
func newnode(p2pCfg config.P2PConfig, mcfg conn.MConnConfig, rs ...p2p.Reactor) (*node, error) {
	port, err := cmtnet.GetFreePort()
	if err != nil {
		return nil, err
	}
	p2pCfg.ListenAddress = fmt.Sprintf("tcp://localhost:%d", port)
	key := ed25519.GenPrivKey()
	n := &node{
		key: key,
		id:  p2p.PubKeyToID(key.PubKey()),
		// cfg:    cfg,
		p2pCfg: p2pCfg,
	}
	addr, err := p2p.NewNetAddressString(p2p.IDAddressString(n.id, p2pCfg.ListenAddress))
	if err != nil {
		return nil, err
	}
	n.addr = addr

	channelIDs := make([]byte, 0)
	for _, r := range rs {
		ch := r.GetChannels()
		for _, c := range ch {
			channelIDs = append(channelIDs, c.ID)
		}
	}

	nodeInfo := p2p.DefaultNodeInfo{
		ProtocolVersion: defaultProtocolVersion,
		ListenAddr:      p2pCfg.ListenAddress,
		DefaultNodeID:   n.id,
		Network:         "test",
		Version:         "1.2.3-rc0-deadbeef",
		Moniker:         "test",
		Channels:        channelIDs,
	}

	mt := p2p.NewMultiplexTransport(
		nodeInfo,
		p2p.NodeKey{PrivKey: key},
		mcfg,
	)

	n.mt = mt

	sw := newSwitch(p2pCfg, mt, rs...)
	n.sw = sw
	return n, nil
}

func (n *node) start() error {
	err := n.mt.Listen(*n.addr)
	if err != nil {
		return err
	}

	if err := n.sw.Start(); err != nil {
		return err
	}
	return nil
}

func (n *node) stop() {
	_ = n.sw.Stop()
	_ = n.mt.Close()
}

func newSwitch(cfg config.P2PConfig, mt *p2p.MultiplexTransport, rs ...p2p.Reactor) *p2p.Switch {
	sw := p2p.NewSwitch(&cfg, mt)
	for i, r := range rs {
		sw.AddReactor(fmt.Sprintf("reactor%d", i), r)
	}
	return sw
}
