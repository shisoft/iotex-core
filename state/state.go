// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package state

import (
	"bytes"
	"encoding/gob"
	"math/big"

	"github.com/iotexproject/iotex-core/pkg/hash"
)

// State is the canonical representation of an account.
type State struct {
	// 0 is reserved from actions in genesis block and coinbase transfers nonces
	// other actions' nonces start from 1
	Nonce        uint64
	Balance      *big.Int
	Root         hash.Hash32B // storage trie root for contract account
	CodeHash     []byte       // hash of the smart contract byte-code for contract account
	IsCandidate  bool
	VotingWeight *big.Int
	Votee        string
	Voters       map[string]*big.Int
}

func stateToBytes(s *State) ([]byte, error) {
	var ss bytes.Buffer
	e := gob.NewEncoder(&ss)
	if err := e.Encode(s); err != nil {
		return nil, ErrFailedToMarshalState
	}
	return ss.Bytes(), nil
}

func bytesToState(ss []byte) (*State, error) {
	var state State
	e := gob.NewDecoder(bytes.NewBuffer(ss))
	if err := e.Decode(&state); err != nil {
		return nil, ErrFailedToUnmarshalState
	}
	return &state, nil
}

// AddBalance adds balance for state
func (st *State) AddBalance(amount *big.Int) error {
	st.Balance.Add(st.Balance, amount)
	return nil
}

// SubBalance subtracts balance for state
func (st *State) SubBalance(amount *big.Int) error {
	// make sure there's enough fund to spend
	if amount.Cmp(st.Balance) == 1 {
		return ErrNotEnoughBalance
	}
	st.Balance.Sub(st.Balance, amount)
	return nil
}

//======================================
// private functions
//======================================
func (st *State) clone() *State {
	s := *st
	s.Balance = nil
	s.Balance = new(big.Int).Set(st.Balance)
	s.VotingWeight = nil
	s.VotingWeight = new(big.Int).Set(st.VotingWeight)
	if st.CodeHash != nil {
		s.CodeHash = nil
		s.CodeHash = make([]byte, len(st.CodeHash))
		copy(s.CodeHash, st.CodeHash)
	}
	// Voters won't be used, set to nil for simplicity
	s.Voters = nil
	return &s
}
