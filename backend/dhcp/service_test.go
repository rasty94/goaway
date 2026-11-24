package dhcp

import (
	"goaway/backend/cluster"
	"goaway/backend/database"
	"goaway/backend/settings"
	"net"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type recordingReplicator struct {
	events []cluster.ReplicatedEvent
}

func (r *recordingReplicator) Broadcast(ev cluster.ReplicatedEvent) {
	r.events = append(r.events, ev)
}

func newTestDHCPService(t *testing.T) (*Service, *recordingReplicator) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&database.StaticDHCPLease{},
		&database.ActiveDHCPLease{},
		&database.StaticDHCPv6Lease{},
		&database.ActiveDHCPv6Lease{},
	))

	cfg := &settings.Config{
		DHCP: settings.DHCPConfig{
			Enabled:       true,
			IPv4Enabled:   true,
			IPv6Enabled:   true,
			RangeStart:    "192.168.10.100",
			RangeEnd:      "192.168.10.105",
			RangeStartV6:  "fd00:10::100",
			RangeEndV6:    "fd00:10::105",
			LeaseDuration: 3600,
			Router:        "192.168.10.1",
			DNSServers:    []string{"192.168.10.2"},
			DNSServersV6:  []string{"fd00:10::2"},
		},
	}

	svc := NewService(NewRepository(db), cfg)
	rep := &recordingReplicator{}
	svc.SetReplicator(rep)
	return svc, rep
}

func TestStaticLeaseValidationAndCRUD(t *testing.T) {
	svc, rep := newTestDHCPService(t)

	assert.Error(t, svc.CreateStaticLease(&database.StaticDHCPLease{MAC: "", IP: "192.168.10.50"}))
	assert.Error(t, svc.CreateStaticLease(&database.StaticDHCPLease{MAC: "not-a-mac", IP: "192.168.10.50"}))
	assert.Error(t, svc.CreateStaticLease(&database.StaticDHCPLease{MAC: "AA:BB:CC:DD:EE:FF", IP: "invalid-ip"}))
	assert.Error(t, svc.CreateStaticLease(&database.StaticDHCPLease{MAC: "AA:BB:CC:DD:EE:FF", IP: "fd00::1"}))

	lease := &database.StaticDHCPLease{
		MAC:      " AA:BB:CC:DD:EE:FF ",
		IP:       " 192.168.10.50 ",
		Hostname: " printer ",
		Enabled:  true,
	}
	require.NoError(t, svc.CreateStaticLease(lease))
	assert.Equal(t, "aa:bb:cc:dd:ee:ff", lease.MAC)
	assert.Equal(t, "192.168.10.50", lease.IP)
	assert.Equal(t, "printer", lease.Hostname)

	list, err := svc.ListStaticLeases()
	require.NoError(t, err)
	require.Len(t, list, 1)

	require.NoError(t, svc.DeleteStaticLease(list[0].ID))
	list, err = svc.ListStaticLeases()
	require.NoError(t, err)
	assert.Empty(t, list)

	require.Len(t, rep.events, 2)
	assert.Equal(t, cluster.EventDHCPStaticAdd, rep.events[0].Type)
	assert.Equal(t, cluster.EventDHCPStaticRemove, rep.events[1].Type)
}

func TestDHCPv4DiscoverRequestReleaseFlow(t *testing.T) {
	svc, _ := newTestDHCPService(t)

	mac, err := net.ParseMAC("00:11:22:33:44:55")
	require.NoError(t, err)

	discover, err := dhcpv4.NewDiscovery(mac)
	require.NoError(t, err)

	offer, err := svc.handleDiscover(discover)
	require.NoError(t, err)
	require.NotNil(t, offer)
	assert.Equal(t, dhcpv4.MessageTypeOffer, offer.MessageType())
	assert.Equal(t, "192.168.10.100", offer.YourIPAddr.String())

	req, err := dhcpv4.NewRequestFromOffer(offer)
	require.NoError(t, err)
	ack, err := svc.handleRequest(req)
	require.NoError(t, err)
	require.NotNil(t, ack)
	assert.Equal(t, dhcpv4.MessageTypeAck, ack.MessageType())
	assert.Equal(t, "192.168.10.100", ack.YourIPAddr.String())

	active, err := svc.ListActiveLeases()
	require.NoError(t, err)
	require.Len(t, active, 1)

	rel, err := dhcpv4.New(
		dhcpv4.WithMessageType(dhcpv4.MessageTypeRelease),
		dhcpv4.WithHwAddr(mac),
		dhcpv4.WithClientIP(offer.YourIPAddr),
	)
	require.NoError(t, err)
	require.NoError(t, svc.handleRelease(rel))
}

func TestDHCPv6SolicitAndRequestFlow(t *testing.T) {
	svc, _ := newTestDHCPService(t)

	mac, err := net.ParseMAC("00:11:22:33:44:66")
	require.NoError(t, err)

	solicit, err := dhcpv6.NewSolicit(mac)
	require.NoError(t, err)

	adv, err := svc.handleSolicit(solicit)
	require.NoError(t, err)
	require.NotNil(t, adv)

	advMsg, err := adv.GetInnerMessage()
	require.NoError(t, err)
	assert.Equal(t, dhcpv6.MessageTypeAdvertise, advMsg.Type())
	iana := advMsg.Options.OneIANA()
	require.NotNil(t, iana)
	addrOpt := iana.Options.OneAddress()
	require.NotNil(t, addrOpt)
	assert.Equal(t, "fd00:10::100", addrOpt.IPv6Addr.String())

	reqMsg, err := dhcpv6.NewMessage()
	require.NoError(t, err)
	reqMsg.MessageType = dhcpv6.MessageTypeRequest
	reqMsg.AddOption(solicit.Options.GetOne(dhcpv6.OptionClientID))
	reqMsg.AddOption(iana)

	reply, err := svc.handleRequestv6(reqMsg)
	require.NoError(t, err)
	require.NotNil(t, reply)

	replyInner, err := reply.GetInnerMessage()
	require.NoError(t, err)
	assert.Equal(t, dhcpv6.MessageTypeReply, replyInner.Type())

	activeV6, err := svc.ListActivev6Leases()
	require.NoError(t, err)
	require.Len(t, activeV6, 1)
}
