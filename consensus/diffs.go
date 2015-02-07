package consensus

// A block is composed of many transactions. Blocks that have transactions that
// depend on other transactions, but the transactions must all be applied in a
// deterministic order. Transactions cannot have inter-dependencies, meaning
// that an output cannot be created and then spent in the same transaction. As
// far as diffs are concenred, this means that the elements of a transaction
// diff should be able to be applied in any order and the outcome should be the
// same. The elements of a block diff however must be applied in a specific
// order, as transactions may depend on each other.

// An OutputDiff indicates an output that has either been added to or removed
// from the unspent outputs set. New=true means that the output was added when
// the block was applied, and new=false means that the output was deleted when
// the block was applied.
type SiacoinOutputDiff struct {
	New           bool
	ID            SiacoinOutputID
	SiacoinOutput SiacoinOutput
}

type FileContractDiff struct {
	New          bool
	ID           FileContractID
	FileContract FileContract
}

type SiafundOutputDiff struct {
	New           bool
	ID            SiafundOutputID
	SiafundOutput SiafundOutput
}

type SiafundPoolDiff struct {
	Previous Currency
	Adjusted Currency
}

// commitOutputDiff takes an output diff and applies it to the state. Forward
// indicates the direction of the blockchain.
func (s *State) commitSiacoinOutputDiff(scod SiacoinOutputDiff, forward bool) {
	add := scod.New
	if !forward {
		add = !add
	}

	if add {
		// Sanity check - output should not already exist.
		if DEBUG {
			_, exists := s.siacoinOutputs[scod.ID]
			if exists {
				panic("rogue new output in applyOutputDiff")
			}
		}

		s.siacoinOutputs[scod.ID] = scod.SiacoinOutput
	} else {
		// Sanity check - output should exist.
		if DEBUG {
			_, exists := s.siacoinOutputs[scod.ID]
			if !exists {
				panic("rogue non-new output in applyOutputDiff")
			}
		}

		delete(s.siacoinOutputs, scod.ID)
	}
}

// commitContractDiff takes a contract diff and applies it to the state. Forward
// indicates the direction of the blockchain.
func (s *State) commitFileContractDiff(fcd FileContractDiff, forward bool) {
	add := fcd.New
	if !forward {
		add = !add
	}

	if add {
		// Sanity check - contract should not already exist.
		if DEBUG {
			_, exists := s.fileContracts[fcd.ID]
			if exists {
				panic("rogue new contract in applyContractDiff")
			}
		}

		s.fileContracts[fcd.ID] = fcd.FileContract
	} else {
		// Sanity check - contract should exist.
		if DEBUG {
			_, exists := s.fileContracts[fcd.ID]
			if !exists {
				panic("rogue non-new contract in applyContractDiff")
			}
		}

		delete(s.fileContracts, fcd.ID)
	}
}

func (s *State) commitSiafundOutputDiff(sfod SiafundOutputDiff, forward bool) {
	add := sfod.New
	if !forward {
		add = !add
	}

	if add {
		// Sanity check - output should not already exist.
		if DEBUG {
			_, exists := s.siafundOutputs[sfod.ID]
			if exists {
				panic("rogue new output in applyOutputDiff")
			}
		}

		s.siafundOutputs[sfod.ID] = sfod.SiafundOutput
	} else {
		// Sanity check - output should exist.
		if DEBUG {
			_, exists := s.siafundOutputs[sfod.ID]
			if !exists {
				panic("rogue non-new output in applyOutputDiff")
			}
		}

		delete(s.siafundOutputs, sfod.ID)
	}
}

func (s *State) commitSiafundPoolDiff(sfpd SiafundPoolDiff, forward bool) {
	if forward {
		s.siafundPool = sfpd.Adjusted
	} else {
		s.siafundPool = sfpd.Previous
	}
}

func (s *State) applyDiffSet(bn *blockNode, direction bool) {
	// Sanity check - diffs should have already been generated for this node.
	if DEBUG {
		if !bn.diffsGenerated {
			panic("misuse of applyDiffSet - diffs have not been generated!")
		}
	}

	// Apply the siacoin, file contract, and siafund diffs.
	s.commitSiafundPoolDiff(bn.siafundPoolDiff, direction)
	for _, scod := range bn.siacoinOutputDiffs {
		s.commitSiacoinOutputDiff(scod, direction)
	}
	for _, fcd := range bn.fileContractDiffs {
		s.commitFileContractDiff(fcd, direction)
	}
	for _, sfod := range bn.siafundOutputDiffs {
		s.commitSiafundOutputDiff(sfod, direction)
	}

	// Manage the delayed outputs that have been created by the node.
	if direction {
		s.delayedSiacoinOutputs[bn.height] = bn.delayedSiacoinOutputs
	} else {
		delete(s.delayedSiacoinOutputs, bn.height)
	}

	// Update the current path and currentBlockID
	if direction {
		s.currentBlockID = bn.block.ID()
		s.currentPath[bn.height] = bn.block.ID()
	} else {
		delete(s.currentPath, bn.height)
		s.currentBlockID = bn.parent.block.ID()
	}
}
