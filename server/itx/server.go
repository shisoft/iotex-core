// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package itx

import (
	"context"
	"os"

	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/blocksync"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus"
	"github.com/iotexproject/iotex-core/dispatch"
	"github.com/iotexproject/iotex-core/dispatch/dispatcher"
	"github.com/iotexproject/iotex-core/explorer"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/network"
	pb "github.com/iotexproject/iotex-core/proto"

	"github.com/pkg/errors"
)

// Server is the iotex server instance containing all components.
type Server struct {
	cfg        *config.Config
	chain      blockchain.Blockchain
	actPool    actpool.ActPool
	p2p        network.Overlay
	consensus  consensus.Consensus
	blocksync  blocksync.BlockSync
	dispatcher dispatcher.Dispatcher
	explorer   *explorer.Server
}

type bcService struct {
	ap actpool.ActPool
	bs blocksync.BlockSync
	cs consensus.Consensus
}

func (bcs *bcService) HandleAction(act *pb.ActionPb) error {
	if pbTsf := act.GetTransfer(); pbTsf != nil {
		tsf := &action.Transfer{}
		tsf.ConvertFromActionPb(act)
		if err := bcs.ap.AddTsf(tsf); err != nil {
			logger.Debug().Err(err)
			return err
		}
	} else if pbVote := act.GetVote(); pbVote != nil {
		vote := &action.Vote{}
		vote.ConvertFromActionPb(act)
		if err := bcs.ap.AddVote(vote); err != nil {
			logger.Debug().Err(err)
			return err
		}
	} else if pbExecution := act.GetExecution(); pbExecution != nil {
		execution := &action.Execution{}
		execution.ConvertFromActionPb(act)
		if err := bcs.ap.AddExecution(execution); err != nil {
			logger.Debug().Err(err).Msg("Failed to add execution")
			return err
		}
	}
	return nil
}

func (bcs *bcService) HandleBlock(blk *blockchain.Block) error {
	return bcs.bs.ProcessBlock(blk)
}

func (bcs *bcService) HandleBlockSync(blk *blockchain.Block) error {
	return bcs.bs.ProcessBlockSync(blk)
}

func (bcs *bcService) HandleSyncRequest(sender string, sync *pb.BlockSync) error {
	return bcs.bs.ProcessSyncRequest(sender, sync)
}

func (bcs *bcService) HandleViewChange(msg proto.Message) error {
	return bcs.cs.HandleViewChange(msg)
}

func (bcs *bcService) HandleBlockPropose(msg proto.Message) error {
	return bcs.cs.HandleBlockPropose(msg)
}

// NewServer creates a new server
func NewServer(cfg *config.Config) *Server {
	// create Blockchain
	chain := blockchain.NewBlockchain(cfg, blockchain.DefaultStateFactoryOption(), blockchain.BoltDBDaoOption())
	if chain == nil && cfg.Chain.EnableFallBackToFreshDB {
		logger.Warn().Msg("Chain db and trie db are falling back to fresh ones")
		if err := os.Rename(cfg.Chain.ChainDBPath, cfg.Chain.ChainDBPath+".old"); err != nil {
			logger.Error().Err(err).Msg("Failed to rename old chain db")
			return nil
		}
		if err := os.Rename(cfg.Chain.TrieDBPath, cfg.Chain.TrieDBPath+".old"); err != nil {
			logger.Error().Err(err).Msg("Failed to rename old trie db")
			return nil
		}
		chain = blockchain.NewBlockchain(cfg, blockchain.DefaultStateFactoryOption(), blockchain.BoltDBDaoOption())

	}
	logger.Error().Uint32("chain id", chain.ChainID()).Msg("Chain ID for new server")
	return newServer(cfg, chain)
}

// NewInMemTestServer creates a test server in memory
func NewInMemTestServer(cfg *config.Config) *Server {
	chain := blockchain.NewBlockchain(cfg, blockchain.InMemStateFactoryOption(), blockchain.InMemDaoOption())
	return newServer(cfg, chain)
}

// Start starts the server
func (s *Server) Start(ctx context.Context) error {
	if err := s.chain.Start(ctx); err != nil {
		return errors.Wrap(err, "error when starting blockchain")
	}
	if err := s.dispatcher.Start(ctx); err != nil {
		return errors.Wrap(err, "error when starting dispatcher")
	}
	if err := s.consensus.Start(ctx); err != nil {
		return errors.Wrap(err, "error when starting consensus")
	}
	if err := s.blocksync.Start(ctx); err != nil {
		return errors.Wrap(err, "error when starting blocksync")
	}
	if err := s.p2p.Start(ctx); err != nil {
		return errors.Wrap(err, "error when starting P2P networks")
	}
	if err := s.explorer.Start(ctx); err != nil {
		return errors.Wrap(err, "error when starting explorer")
	}
	return nil
}

// Stop stops the server
func (s *Server) Stop(ctx context.Context) error {
	if err := s.explorer.Stop(ctx); err != nil {
		return errors.Wrap(err, "error when stopping explorer")
	}
	if err := s.p2p.Stop(ctx); err != nil {
		return errors.Wrap(err, "error when stopping P2P networks")
	}
	if err := s.consensus.Stop(ctx); err != nil {
		return errors.Wrap(err, "error when stopping consensus")
	}
	if err := s.blocksync.Stop(ctx); err != nil {
		return errors.Wrap(err, "error when stopping blocksync")
	}
	if err := s.dispatcher.Stop(ctx); err != nil {
		return errors.Wrap(err, "error when stopping dispatcher")
	}
	if err := s.chain.Stop(ctx); err != nil {
		return errors.Wrap(err, "error when stopping blockchain")
	}
	return nil
}

// Blockchain returns the Blockchain
func (s *Server) Blockchain() blockchain.Blockchain {
	return s.chain
}

// ActionPool returns the Action pool
func (s *Server) ActionPool() actpool.ActPool {
	return s.actPool
}

// P2P returns the P2P network
func (s *Server) P2P() network.Overlay {
	return s.p2p
}

// Dispatcher returns the Dispatcher
func (s *Server) Dispatcher() dispatcher.Dispatcher {
	return s.dispatcher
}

// Consensus returns the consensus instance
func (s *Server) Consensus() consensus.Consensus {
	return s.consensus
}

// BlockSync returns the block syncer
func (s *Server) BlockSync() blocksync.BlockSync {
	return s.blocksync
}

// Explorer returns the explorer instance
func (s *Server) Explorer() *explorer.Server {
	return s.explorer
}

func newServer(cfg *config.Config, chain blockchain.Blockchain) *Server {
	// create P2P network and BlockSync
	p2p := network.NewOverlay(&cfg.Network)
	// Create ActPool
	actPool, err := actpool.NewActPool(chain, cfg.ActPool)
	if err != nil {
		logger.Fatal().Err(err).Msg("Fail to create actpool")
	}
	bs, err := blocksync.NewBlockSyncer(cfg, chain, actPool, p2p)
	if err != nil {
		logger.Fatal().Err(err).Msg("Fail to create blockSyncer")
	}
	consensus := consensus.NewConsensus(cfg, chain, actPool, p2p)
	if consensus == nil {
		logger.Fatal().Msg("Failed to create Consensus")
	}
	// create dispatcher instance
	dispatcher, err := dispatch.NewDispatcher(cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("Fail to create dispatcher")
	}
	dispatcher.AddSubscriber(chain.ChainID(), &bcService{actPool, bs, consensus})
	p2p.AttachDispatcher(dispatcher)
	var exp *explorer.Server
	if cfg.Explorer.IsTest || os.Getenv("APP_ENV") == "development" {
		logger.Warn().Msg("Using test server with fake data...")
		exp = explorer.NewTestSever(cfg.Explorer)
	} else {
		exp = explorer.NewServer(cfg.Explorer, chain, consensus, dispatcher, actPool, p2p)
	}

	return &Server{
		cfg:        cfg,
		chain:      chain,
		actPool:    actPool,
		p2p:        p2p,
		consensus:  consensus,
		blocksync:  bs,
		dispatcher: dispatcher,
		explorer:   exp,
	}
}