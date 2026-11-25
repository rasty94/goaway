package server

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"io"
	"testing"
	"time"

	"codeberg.org/miekg/dns"
	"github.com/quic-go/quic-go"
	"github.com/stretchr/testify/require"
)

// TestDoQServerReleasesStreamsWhenFINArrivesLate opens more streams than the
// server's concurrent-stream limit (quic-go default 100) on one connection,
// sending each query's FIN only after the answer was read. The server must
// still return stream credit, or OpenStreamSync blocks once the limit is hit.
func TestDoQServerReleasesStreamsWhenFINArrivesLate(t *testing.T) {
	cert, pool := generateSelfSignedCert(t)
	s := &DNSServer{}
	srv, err := s.InitDoQWithAddr("127.0.0.1:0", cert, stubHandler)
	require.NoError(t, err)
	go func() { _ = srv.ListenAndServe() }()
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := quic.DialAddr(ctx, srv.Addr(), &tls.Config{
		RootCAs: pool, ServerName: "127.0.0.1", NextProtos: []string{doqALPN}, MinVersion: tls.VersionTLS13,
	}, nil)
	require.NoError(t, err)
	defer func() { _ = conn.CloseWithError(0, "") }()

	q := dns.NewMsg("late-fin.example.com.", dns.TypeA)
	q.ID = 0
	require.NoError(t, q.Pack())
	framed := binary.BigEndian.AppendUint16(nil, uint16(len(q.Data))) //nolint:gosec // tiny test query
	framed = append(framed, q.Data...)

	for i := range 150 {
		stream, err := conn.OpenStreamSync(ctx)
		require.NoError(t, err, "stream %d: server stopped granting streams", i)
		_, err = stream.Write(framed)
		require.NoError(t, err)

		var lenBuf [2]byte
		_, err = io.ReadFull(stream, lenBuf[:])
		require.NoError(t, err, "stream %d", i)
		_, err = io.ReadFull(stream, make([]byte, binary.BigEndian.Uint16(lenBuf[:])))
		require.NoError(t, err, "stream %d", i)

		// FIN only now, after the server is done. The server may already have
		// sent STOP_SENDING (it cancels its read side), so Close can fail.
		_ = stream.Close()
		stream.CancelRead(doqNoError)
	}
}
