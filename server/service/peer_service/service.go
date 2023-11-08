package peer_service

import (
	context "context"
	"crypto/rand"
	"errors"
	"fmt"
	"github.com/gofiber/fiber/v2/log"
	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	"github.com/opennoty/opennoty/pkg/mongo_kvstore"
	"github.com/opennoty/opennoty/server/api/peer_proto"
	peer_transport2 "github.com/opennoty/opennoty/server/peer_transport"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"net"
	"os"
	"sync"
	"time"
)

type PayloadHandler func(p *Peer, payload *peer_proto.Payload)

type peerService struct {
	appCtx          context.Context
	mongoDatabase   *mongo.Database
	peersCollection *mongo.Collection

	peerId            peer.ID
	peerNotifyAddress string
	peerPort          int
	hostname          string

	localPeerDocument  *PeerDocument
	registerSelfTicker *time.Ticker
	mongoStore         *mongo_kvstore.MongoKVStore[*PeerDocument]

	noise  *noise.Transport
	server *peer_transport2.Server

	lock sync.RWMutex
	// peers protect by lock
	peers map[string]*Peer

	payloadHandlers map[peer_proto.PayloadType]PayloadHandler
}

func New() (PeerService, error) {
	privKey, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, err
	}
	peerId, err := peer.IDFromPrivateKey(privKey)
	if err != nil {
		return nil, err
	}
	noiseTransport, err := noise.New(noise.ID, privKey, nil)
	if err != nil {
		return nil, err
	}

	s := &peerService{
		peerId:          peerId,
		noise:           noiseTransport,
		peers:           map[string]*Peer{},
		payloadHandlers: map[peer_proto.PayloadType]PayloadHandler{},
	}

	log.Infof("Local PeerId: %s", peerId.String())

	return s, nil
}

func (s *peerService) Start(appCtx context.Context, listenAddress string, peerNotifyAddress string, mongoDatabase *mongo.Database) error {
	var err error

	s.appCtx = appCtx
	s.mongoDatabase = mongoDatabase
	s.peersCollection = mongoDatabase.Collection("opennoty.peers")
	s.peerNotifyAddress = peerNotifyAddress

	if s.peerNotifyAddress == "" {
		s.peerNotifyAddress = findIp()
	}

	s.hostname, _ = os.Hostname()

	listener, err := net.Listen("tcp4", listenAddress)
	if err != nil {
		return err
	}

	addr := listener.Addr().(*net.TCPAddr)
	s.peerPort = addr.Port

	if err = s.initializeMongo(); err != nil {
		return err
	}
	if err = s.startMongoPeer(); err != nil {
		log.Errorf("registerSelf failed: %v", err)
		return err
	}

	log.Infof("cluster server listening: %s", listener.Addr())

	s.server = peer_transport2.NewServer(s.appCtx, s.noise)
	go s.acceptLoop(listener)

	return nil
}

func (s *peerService) Stop() {
	if s.registerSelfTicker != nil {
		s.registerSelfTicker.Stop()
		s.registerSelfTicker = nil
	}

	if s.localPeerDocument != nil {
		result, err := s.peersCollection.DeleteMany(context.Background(), bson.M{
			"_id": s.localPeerDocument.Id,
		})
		log.Info(result)
		log.Info(err)
	}
}

func (s *peerService) RegisterPayloadHandler(payloadType peer_proto.PayloadType, handler PayloadHandler) {
	if s.payloadHandlers[payloadType] != nil {
		panic(errors.New(fmt.Sprintf("already registered: %s", payloadType)))
	}

	s.payloadHandlers[payloadType] = handler
}

func (s *peerService) GetPeerId() string {
	return s.peerId.String()
}

func (s *peerService) GetPeers() []*Peer {
	var peers []*Peer
	s.lock.RLock()
	defer s.lock.RUnlock()
	for _, p := range s.peers {
		peers = append(peers, p)
	}
	return peers
}

func (s *peerService) NotifyTopic(peerId string, tenantId string, topicName string, data []byte) error {
	p, ok := s.peers[peerId]
	if !ok {
		return fmt.Errorf("no peer %s", peerId)
	}

	payload := NewPayload(peer_proto.PayloadType_kPayloadTopicNotify)
	payload.TenantId = tenantId
	payload.TopicName = topicName
	payload.TopicData = data

	return p.Write(payload)
}

func (s *peerService) initializeMongo() error {
	_, err := s.peersCollection.Indexes().CreateMany(context.Background(), peerIndexes)
	if err != nil {
		return err
	}
	return nil
}

func (s *peerService) startMongoPeer() error {
	if err := s.registerSelf(); err != nil {
		return err
	}
	s.registerSelfTicker = time.NewTicker(time.Second * 10)
	go func() {
		for t := range s.registerSelfTicker.C {
			err := s.registerSelf()
			if err != nil {
				log.Errorf("registerSelf failed: %v", err)
			}
			_ = t
		}
	}()

	s.mongoStore = mongo_kvstore.NewMongoKVStore[*PeerDocument]()
	s.mongoStore.UseStore = false
	s.mongoStore.NewDocument = func() *PeerDocument {
		return &PeerDocument{}
	}
	s.mongoStore.Handler = s

	if err := s.mongoStore.Start(s.appCtx, s.peersCollection); err != nil {
		return err
	}

	return nil
}

func (s *peerService) OnInserted(doc *PeerDocument) {
	s.checkAndConnectTo(doc)
}

func (s *peerService) OnUpdated(doc *PeerDocument) {
}

func (s *peerService) OnDeleted(key string, doc *PeerDocument) {
	s.removePeerByMongoId(key)
}

func (s *peerService) registerSelf() error {
	if s.localPeerDocument == nil {
		s.localPeerDocument = &PeerDocument{
			PeerId:  s.peerId.String(),
			Address: s.peerNotifyAddress,
			Port:    s.peerPort,
		}
	}
	s.localPeerDocument.HeartbeatAt = time.Now()
	ctx := context.Background()

	filterDoc := s.localPeerDocument.ToFilter()
	newId := primitive.NewObjectID()
	updateDoc, err := s.localPeerDocument.ToUpdate(newId)
	if err != nil {
		return err
	}
	result := s.peersCollection.FindOneAndUpdate(ctx, filterDoc, updateDoc, options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After))
	err = result.Err()
	if err != nil {
		return err
	}

	if err = result.Decode(s.localPeerDocument); err != nil {
		return err
	}

	return nil
}

func (s *peerService) getPeer(peerId peer.ID) *Peer {
	peerIdStr := peerId.String()

	s.lock.Lock()
	defer s.lock.Unlock()
	p := s.peers[peerIdStr]
	if p == nil {
		p = &Peer{
			peerId:  peerId,
			state:   PeerNotConnected,
			enabled: true,
		}
		s.peers[peerIdStr] = p
		return p
	}
	return p
}

func (s *peerService) findPeer(peerId string) *Peer {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.peers[peerId]
}

func (s *peerService) acceptLoop(listener net.Listener) {
	for s.appCtx.Err() == nil {
		conn, err := listener.Accept()
		if err != nil {
			log.Infof("accept failed: ", err)
			break
		}

		go s.clientIncoming(conn)
	}
}

func (s *peerService) clientIncoming(insecureConn net.Conn) {
	log.Debugf("Peer[address=%s] accept client", insecureConn.RemoteAddr())

	connection, err := s.server.Handshake(insecureConn)
	if err != nil {
		log.Warnf("Peer[address=%s] handshake failed: %v", insecureConn.RemoteAddr(), err)
		return
	}

	p := s.getPeer(connection.RemotePeer())
	p.lock.Lock()
	if p.state != PeerNotConnected {
		p.lock.Unlock()
		log.Debugf("Peer[address=%s, id=%s] already connected", insecureConn.RemoteAddr(), p.peerId)
		// Already connected
		connection.Close()
		return
	}
	p.Connected(connection)
	p.lock.Unlock()

	log.Infof("Peer[address=%s, id=%s] connected with in-coming", insecureConn.RemoteAddr(), p.peerId)

	p.peerId = connection.RemotePeer()
	s.protocolHandler(p, connection)
}

func (s *peerService) checkAndConnectTo(peerDocument *PeerDocument) {
	if peerDocument.PeerId == s.localPeerDocument.PeerId {
		return
	}

	fullAddress := fmt.Sprintf("%s:%d", peerDocument.Address, peerDocument.Port)

	peerId, err := peer.Decode(peerDocument.PeerId)
	if err != nil {
		log.Warnf("checkAndConnectTo(%s) failed: %v", fullAddress, err)
		return
	}

	p := s.getPeer(peerId)
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.state != PeerNotConnected {
		// Already connected
		return
	}

	p.state = PeerConnecting
	p.peerDocument = *peerDocument
	p.fullAddress = fullAddress
	p.peerId = peerId

	go s.connectTo(p)
}

func (s *peerService) connectTo(p *Peer) {
	if p.state != PeerConnecting {
		panic(fmt.Errorf("assert p.state == PeerConnecting, but state is %v", p.state))
	}

	ctx, _ := context.WithTimeout(context.Background(), time.Second*3)
	connection, err := peer_transport2.Connect(ctx, s.noise, p.fullAddress, p.peerId)
	if err != nil {
		p.Close()
		log.Infof("Peer[%s] connect failed (%s)", p.peerId.String(), p.fullAddress)
		return
	}

	log.Infof("Peer[%s] connected with out-coming (%s)", p.peerId.String(), p.fullAddress)

	p.Connected(connection)
	s.protocolHandler(p, connection)
}

func (s *peerService) onPeerClosed(p *Peer) {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.peers, p.peerId.String())
}

func (s *peerService) removePeerByMongoId(id string) {
	var targetPeer *Peer

	s.lock.Lock()
	defer s.lock.Unlock()

	for _, p := range s.peers {
		if p.peerDocument.Id.String() == id {
			targetPeer = p
			break
		}
	}
	if targetPeer != nil {
		targetPeer.enabled = false
		log.Debugf("Peer[%s] removed from PeerStore", targetPeer.peerId)
		delete(s.peers, targetPeer.peerId.String())
		targetPeer.Close()
	}
}

func NewPayload(payloadType peer_proto.PayloadType) *peer_proto.Payload {
	requestId := uuid.New()
	requestIdRaw, _ := requestId.MarshalBinary()
	return &peer_proto.Payload{
		Type:      payloadType,
		RequestId: requestIdRaw,
	}
}
