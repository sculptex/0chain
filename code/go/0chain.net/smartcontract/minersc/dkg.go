package minersc

import (
	"errors"
	"fmt"
	"log"
	"math"
	"reflect"
	"runtime"
	"sort"

	"0chain.net/chaincore/block"
	c_state "0chain.net/chaincore/chain/state"
	"0chain.net/chaincore/node"
	"0chain.net/chaincore/transaction"
	"0chain.net/core/common"
	. "0chain.net/core/logging"
	"0chain.net/core/util"
	"go.uber.org/zap"
)

var moveFunctions = make(map[int]movePhaseFunctions)

func (msc *MinerSmartContract) moveToContribute(balances c_state.StateContextI, pn *PhaseNode, gn *globalNode) (result bool) {
	var err error
	var allMinersList *MinerNodes
	var dkgMinersList *DKGMinerNodes

	allMinersList, err = msc.GetMinersList(balances)
	if err != nil {
		return false
	}

	dkgMinersList, err = msc.getMinersDKGList(balances)
	if err != nil {
		return false
	}
	return allMinersList != nil && len(allMinersList.Nodes) >= dkgMinersList.K
}

func (msc *MinerSmartContract) moveToShareOrPublish(balances c_state.StateContextI, pn *PhaseNode, gn *globalNode) (result bool) {
	var err error
	var dkgMinersList *DKGMinerNodes
	mpks := block.NewMpks()

	dkgMinersList, err = msc.getMinersDKGList(balances)
	if err != nil {
		Logger.Error("failed to get miners DKG", zap.Any("error", err), zap.Any("phase", pn.Phase))
		return false
	}

	msc.mutexMinerMPK.Lock()
	defer msc.mutexMinerMPK.Unlock()

	var mpksBytes util.Serializable
	mpksBytes, err = balances.GetTrieNode(MinersMPKKey)
	if err != nil {
		Logger.Error("failed to get node MinersMPKKey", zap.Any("error", err), zap.Any("phase", pn.Phase))
		return false
	}

	if err = mpks.Decode(mpksBytes.Encode()); err != nil {
		Logger.Error("failed to decode node MinersMPKKey", zap.Any("error", err), zap.Any("phase", pn.Phase))
		return false
	}

	return mpks != nil && len(mpks.Mpks) >= dkgMinersList.K
}

func (msc *MinerSmartContract) moveToWait(balances c_state.StateContextI, pn *PhaseNode, gn *globalNode) (result bool) {
	var err error
	var dkgMinersList *DKGMinerNodes
	gsos := block.NewGroupSharesOrSigns()

	dkgMinersList, err = msc.getMinersDKGList(balances)
	if err != nil {
		Logger.Error("failed to get miners DKG", zap.Any("error", err), zap.Any("phase", pn.Phase))
		return false
	}

	var groupBytes util.Serializable
	groupBytes, err = balances.GetTrieNode(GroupShareOrSignsKey)
	if err != nil {
		Logger.Error("failed to get node GroupShareOrSignsKey", zap.Any("error", err), zap.Any("phase", pn.Phase))
		return false
	}

	if err = gsos.Decode(groupBytes.Encode()); err != nil {
		Logger.Error("failed to decode node GroupShareOrSignsKey", zap.Any("error", err), zap.Any("phase", pn.Phase))
		return false
	}
	return len(gsos.Shares) >= dkgMinersList.K
}

func (msc *MinerSmartContract) moveToStart(balances c_state.StateContextI, pn *PhaseNode, gn *globalNode) bool {
	return true
}

func (msc *MinerSmartContract) getPhaseNode(statectx c_state.StateContextI) (*PhaseNode, error) {
	pn := &PhaseNode{}
	phaseNodeBytes, err := statectx.GetTrieNode(pn.GetKey())
	if err != nil && err != util.ErrValueNotPresent {
		return nil, err
	}
	if phaseNodeBytes == nil {
		pn.Phase = Start
		pn.CurrentRound = statectx.GetBlock().Round
		pn.StartRound = statectx.GetBlock().Round
		return pn, nil
	}
	pn.Decode(phaseNodeBytes.Encode())
	pn.CurrentRound = statectx.GetBlock().Round
	return pn, nil
}

func (msc *MinerSmartContract) setPhaseNode(statectx c_state.StateContextI, pn *PhaseNode, gn *globalNode) error {
	//log.Println("setPhaseNode current round", pn.CurrentRound, "startround", pn.StartRound, "phase", pn.Phase)
	if pn.CurrentRound-pn.StartRound >= PhaseRounds[pn.Phase] {
		currentMoveFunc := moveFunctions[pn.Phase]
		if currentMoveFunc(statectx, pn, gn) {
			var err error
			if phaseFunc, ok := phaseFuncs[pn.Phase]; ok {
				if lock, found := lockPhaseFunctions[pn.Phase]; found {
					lock.Lock()
					err = phaseFunc(statectx, gn)
					lock.Unlock()
				} else {
					err = phaseFunc(statectx, gn)
				}

				if err != nil {
					msc.RestartDKG(pn, statectx)
					Logger.Error("failed to set phase node", zap.Any("error", err), zap.Any("phase", pn.Phase))
				}
			}
			if err == nil {
				if len(PhaseRounds)-1 > pn.Phase {
					pn.Phase++
				} else {
					pn.Phase = 0
					pn.Restarts = 0
				}
				pn.StartRound = pn.CurrentRound
				if msc.CallbackPhase != nil {
					log.Println("trying CallbackPhase ")
					msc.CallbackPhase(pn.Phase)
				}
			}
		} else {
			Logger.Warn("failed to move phase", zap.Any("phase", pn.Phase),
				zap.Any("move_func", getFunctionName(currentMoveFunc)))
			msc.RestartDKG(pn, statectx)
		}
	}
	_, err := statectx.InsertTrieNode(pn.GetKey(), pn)
	if err != nil && err != util.ErrValueNotPresent {
		Logger.DPanic("failed to set phase node -- insert failed", zap.Any("error", err))
		return err
	}
	return nil
}

func (msc *MinerSmartContract) createDKGMinersForContribute(balances c_state.StateContextI, gn *globalNode) error {
	allminerslist, err := msc.GetMinersList(balances)
	if err != nil {
		Logger.Error("createDKGMinersForContribute -- failed to get miner list", zap.Any("error", err))
		return err
	}
	var n int
	if len(allminerslist.Nodes) < gn.MinN {
		return common.NewError("failed to create dkg miners", "too few miners for dkg")
	}
	if len(allminerslist.Nodes) > gn.MaxN {
		n = gn.MaxN
		sort.Slice(allminerslist.Nodes, func(i, j int) bool {
			log.Println("createDKGMinersForContribute miner", allminerslist.Nodes[i].N2NHost, "stake", allminerslist.Nodes[i].TotalStaked)
			return allminerslist.Nodes[i].TotalStaked > allminerslist.Nodes[j].TotalStaked
		})
	} else {
		n = len(allminerslist.Nodes)
	}
	dkgMiners := NewDKGMinerNodes()
	dkgMiners.N = n
	dkgMiners.K = int(math.Ceil(gn.KPercent * float64(n)))
	dkgMiners.T = int(math.Ceil(gn.TPercent * float64(n)))
	for _, node := range allminerslist.Nodes {
		dkgMiners.SimpleNodes[node.ID] = node.SimpleNode
		if len(dkgMiners.SimpleNodes) == dkgMiners.N {
			break
		}
	}
	_, err = balances.InsertTrieNode(DKGMinersKey, dkgMiners)
	if err != nil {
		return err
	}

	//sharders
	/*allShardersList, err := msc.getShardersList(balances, ShardersKeepKey)
	if err != nil {
		Logger.Error("createDKGMinersForContribute -- failed to get sharder list", zap.Any("error", err))
		return err
	}*/
	allSharderKeepList := NewMinerNode()
	_, err = balances.InsertTrieNode(ShardersKeepKey, allSharderKeepList)
	if err != nil {
		return err
	}
	return nil
}

func (msc *MinerSmartContract) widdleDKGMinersForShare(balances c_state.StateContextI, gn *globalNode) error {
	log.Println("widdleDKGMinersForShare")

	dkgMiners, err := msc.getMinersDKGList(balances)
	if err != nil {
		Logger.Error("widdle dkg miners -- failed to get dkgMiners", zap.Any("error", err))
		return err
	}

	msc.mutexMinerMPK.Lock()
	defer msc.mutexMinerMPK.Unlock()

	mpks := block.NewMpks()
	mpksBytes, err := balances.GetTrieNode(MinersMPKKey)
	if err != nil {
		Logger.Error("widdle dkg miners -- failed to get miners mpks", zap.Any("error", err))
		return err
	}
	if err = mpks.Decode(mpksBytes.Encode()); err != nil {
		return err
	}
	for k := range dkgMiners.SimpleNodes {
		if _, ok := mpks.Mpks[k]; !ok {
			delete(dkgMiners.SimpleNodes, k)
		}
	}
	_, err = balances.InsertTrieNode(DKGMinersKey, dkgMiners)
	if err != nil {
		Logger.Error("widdle dkg miners -- failed to insert dkg miners", zap.Any("error", err))
		return err
	}
	return nil
}

func (msc *MinerSmartContract) createMagicBlockForWait(balances c_state.StateContextI, gn *globalNode) error {
	pn, err := msc.getPhaseNode(balances)
	if err != nil {
		return err
	}
	dkgMinersList, err := msc.getMinersDKGList(balances)
	if err != nil {
		return err
	}
	gsos := block.NewGroupSharesOrSigns()
	groupBytes, err := balances.GetTrieNode(GroupShareOrSignsKey)
	if err != nil {
		return err
	}
	if err = gsos.Decode(groupBytes.Encode()); err != nil {
		return err
	}

	msc.mutexMinerMPK.Lock()
	defer msc.mutexMinerMPK.Unlock()

	mpks := block.NewMpks()
	mpksBytes, err := balances.GetTrieNode(MinersMPKKey)
	if err != nil {
		return common.NewErrorf("create_magic_block_failed", "error with miner's mpk: %v", err)
	}
	if err = mpks.Decode(mpksBytes.Encode()); err != nil {
		return err
	}

	for key := range mpks.Mpks {
		if _, ok := gsos.Shares[key]; !ok {
			delete(dkgMinersList.SimpleNodes, key)
			delete(gsos.Shares, key)
			delete(mpks.Mpks, key)
		}
	}
	for key, sharesRevealed := range dkgMinersList.RevealedShares {
		if sharesRevealed == dkgMinersList.N {
			delete(dkgMinersList.SimpleNodes, key)
			delete(gsos.Shares, key)
			delete(mpks.Mpks, key)
		}
	}

	// sharders
	allSharderList, err := msc.getShardersList(balances, AllShardersKey)
	if err != nil {
		return err
	}
	sharders, err := msc.getShardersList(balances, ShardersKeepKey)
	if err != nil {
		return err
	}

	lmb := balances.GetLastestFinalizedMagicBlock()
	activeSharders := lmb.MagicBlock.Sharders.CopyNodesMap()
	for _, checkNode := range allSharderList.Nodes {
		log.Println("check node", checkNode.N2NHost, checkNode.ID)
		if sharders.FindNodeById(checkNode.ID) != nil {
			log.Println("check node exists")
			continue
		}
		if _, active := activeSharders[checkNode.ID]; !active {
			log.Println("check node in activeSharders")
			sharders.Nodes = append(sharders.Nodes, checkNode)
		} else {
			log.Println("check node in activeSharders")
		}
	}

	log.Println("new mb with sharders", sharders, "all", allSharderList, "active", activeSharders)

	magicBlock, err := msc.CreateMagicBlock(balances, sharders, dkgMinersList, gsos, mpks, pn)
	if err != nil {
		return err
	}

	gn.ViewChange = magicBlock.StartingRound
	mpks = block.NewMpks()
	_, err = balances.InsertTrieNode(MinersMPKKey, mpks)
	if err != nil {
		return err
	}
	gsos = block.NewGroupSharesOrSigns()
	_, err = balances.InsertTrieNode(GroupShareOrSignsKey, gsos)
	if err != nil {
		return err
	}
	_, err = balances.InsertTrieNode(MagicBlockKey, magicBlock)
	if err != nil {
		Logger.Error("failed to insert magic block", zap.Any("error", err))
		return err
	}
	dkgMinersList = NewDKGMinerNodes()
	_, err = balances.InsertTrieNode(DKGMinersKey, dkgMinersList)
	if err != nil {
		return err
	}
	allMinersList := NewMinerNode()
	_, err = balances.InsertTrieNode(ShardersKeepKey, allMinersList)
	if err != nil {
		return err
	}
	return nil
}

func (msc *MinerSmartContract) contributeMpk(t *transaction.Transaction, inputData []byte,
	gn *globalNode, balances c_state.StateContextI) (result string, err error) {
	pn, err := msc.getPhaseNode(balances)
	if err != nil {
		return "", err
	}
	if pn.Phase != Contribute {
		return "", common.NewError("contribute_mpk_failed", "this is not the correct phase to contribute mpk")
	}
	dmn, err := msc.getMinersDKGList(balances)
	if err != nil {
		return "", err
	}
	if _, ok := dmn.SimpleNodes[t.ClientID]; !ok {
		return "", common.NewError("contribute_mpk_failed", "miner not part of dkg set")
	}

	msc.mutexMinerMPK.Lock()
	defer msc.mutexMinerMPK.Unlock()

	mpks := block.NewMpks()
	mpk := &block.MPK{ID: t.ClientID}
	mpksBytes, err := balances.GetTrieNode(MinersMPKKey)
	if mpksBytes != nil {
		if err = mpks.Decode(mpksBytes.Encode()); err != nil {
			return "", err
		}
	}
	err = mpk.Decode(inputData)
	if err != nil {
		return "", err
	}
	if len(mpk.Mpk) != dmn.T {
		return "", common.NewError("contribute_mpk_failed", fmt.Sprintf("mpk sent (size: %v) is not correct size: %v", len(mpk.Mpk), dmn.T))
	}
	if _, ok := mpks.Mpks[mpk.ID]; ok {
		return "", common.NewError("contribute_mpk_failed", "already have mpk for miner")
	}
	mpks.Mpks[mpk.ID] = mpk
	_, err = balances.InsertTrieNode(MinersMPKKey, mpks)
	if err != nil {
		return "", err
	}
	return string(mpk.Encode()), nil
}

func (msc *MinerSmartContract) shareSignsOrShares(t *transaction.Transaction, inputData []byte, gn *globalNode, balances c_state.StateContextI) (string, error) {

	pn, err := msc.getPhaseNode(balances)
	if err != nil {
		return "", err
	}
	if pn.Phase != Publish {
		return "", common.NewError("share_signs_or_shares", fmt.Sprintf("this is not the correct phase to publish signs or shares, phase node: %v", string(pn.Encode())))
	}
	gsos := block.NewGroupSharesOrSigns()
	groupBytes, err := balances.GetTrieNode(GroupShareOrSignsKey)
	if groupBytes != nil {
		gsos.Decode(groupBytes.Encode())
	}
	if _, ok := gsos.Shares[t.ClientID]; ok {
		return "", common.NewError("share_signs_or_shares", fmt.Sprintf("already have share or signs for miner %v", t.ClientID))
	}
	dmn, err := msc.getMinersDKGList(balances)
	if err != nil {
		return "", err
	}
	sos := block.NewShareOrSigns()
	err = sos.Decode(inputData)
	if err != nil {
		return "", nil
	}
	if len(sos.ShareOrSigns) < dmn.N-2 {
		return "", common.NewError("failed to add share or signs", "number of share or signs doesn't equal N for this dkg")
	}

	msc.mutexMinerMPK.Lock()
	defer msc.mutexMinerMPK.Unlock()

	mpks := block.NewMpks()
	mpksBytes, err := balances.GetTrieNode(MinersMPKKey)
	if err != nil {
		return "", err
	}
	mpks.Decode(mpksBytes.Encode())
	publicKeys := make(map[string]string)
	for key, miner := range dmn.SimpleNodes {
		publicKeys[key] = miner.PublicKey
	}
	shares, ok := sos.Validate(mpks, publicKeys, balances.GetSignatureScheme())
	if !ok {
		return "", common.NewError("failed to add share or sign", "share or signs failed validation")
	}
	for _, share := range shares {
		dmn.RevealedShares[share]++
	}
	sos.ID = t.ClientID
	gsos.Shares[t.ClientID] = sos
	_, err = balances.InsertTrieNode(GroupShareOrSignsKey, gsos)
	if err != nil {
		return "", err
	}
	_, err = balances.InsertTrieNode(DKGMinersKey, dmn)
	if err != nil {
		return "", err
	}
	return string(sos.Encode()), nil
}

func (msc *MinerSmartContract) getMinersDKGList(statectx c_state.StateContextI) (*DKGMinerNodes, error) {
	allMinersList := NewDKGMinerNodes()
	allMinersBytes, err := statectx.GetTrieNode(DKGMinersKey)
	if err != nil && err != util.ErrValueNotPresent {
		return nil, errors.New("getMinersList_failed - Failed to retrieve existing miners list")
	}
	if allMinersBytes == nil {
		return allMinersList, nil
	}
	allMinersList.Decode(allMinersBytes.Encode())
	return allMinersList, nil
}

func (msc *MinerSmartContract) CreateMagicBlock(balances c_state.StateContextI, sharderList *MinerNodes, dkgMinersList *DKGMinerNodes, gsos *block.GroupSharesOrSigns, mpks *block.Mpks, pn *PhaseNode) (*block.MagicBlock, error) {
	magicBlock := block.NewMagicBlock()
	magicBlock.Miners = node.NewPool(node.NodeTypeMiner)
	magicBlock.Sharders = node.NewPool(node.NodeTypeSharder)
	magicBlock.SetShareOrSigns(gsos)
	magicBlock.Mpks = mpks
	magicBlock.T = dkgMinersList.T
	magicBlock.K = dkgMinersList.K
	magicBlock.N = dkgMinersList.N
	for _, v := range dkgMinersList.SimpleNodes {
		n := &node.Node{}
		n.ID = v.ID
		n.N2NHost = v.N2NHost
		n.Host = v.Host
		n.Port = v.Port
		n.PublicKey = v.PublicKey
		n.Description = v.ShortName
		n.Type = node.NodeTypeMiner
		n.Info.BuildTag = v.BuildTag
		n.Status = node.NodeStatusActive
		magicBlock.Miners.AddNode(n)
	}
	prevMagicBlock := balances.GetLastestFinalizedMagicBlock()
	/*sharders, err := msc.getShardersList(balances, AllShardersKey)
	if err != nil {
		return nil, err
	}*/
	for _, v := range sharderList.Nodes {
		n := &node.Node{}
		n.ID = v.ID
		n.N2NHost = v.N2NHost
		n.Host = v.Host
		n.Port = v.Port
		n.PublicKey = v.PublicKey
		n.Description = v.ShortName
		n.Type = node.NodeTypeSharder
		n.Info.BuildTag = v.BuildTag
		n.Status = node.NodeStatusActive
		magicBlock.Sharders.AddNode(n)
	}
	magicBlock.MagicBlockNumber = prevMagicBlock.MagicBlock.MagicBlockNumber + 1
	magicBlock.PreviousMagicBlockHash = prevMagicBlock.MagicBlock.Hash
	magicBlock.StartingRound = pn.CurrentRound + PhaseRounds[Wait]
	magicBlock.Hash = magicBlock.GetHash()
	return magicBlock, nil
}

func (msc *MinerSmartContract) RestartDKG(pn *PhaseNode, balances c_state.StateContextI) {
	msc.mutexMinerMPK.Lock()
	defer msc.mutexMinerMPK.Unlock()
	mpks := block.NewMpks()
	_, err := balances.InsertTrieNode(MinersMPKKey, mpks)
	if err != nil {
		Logger.Error("failed to restart dkg", zap.Any("error", err))
	}
	gsos := block.NewGroupSharesOrSigns()
	_, err = balances.InsertTrieNode(GroupShareOrSignsKey, gsos)
	if err != nil {
		Logger.Error("failed to restart dkg", zap.Any("error", err))
	}
	dkgMinersList := NewDKGMinerNodes()
	_, err = balances.InsertTrieNode(DKGMinersKey, dkgMinersList)
	if err != nil {
		Logger.Error("failed to restart dkg", zap.Any("error", err))
	}

	allMinersList := NewMinerNode()
	_, err = balances.InsertTrieNode(ShardersKeepKey, allMinersList)
	if err != nil {
		Logger.Error("failed to restart dkg", zap.Any("error", err))
	}
	pn.Phase = Start
	pn.Restarts++
	pn.StartRound = pn.CurrentRound
}

func (msc *MinerSmartContract) SetMagicBlock(balances c_state.StateContextI) bool {
	magicBlockBytes, err := balances.GetTrieNode(MagicBlockKey)
	if err != nil {
		Logger.Error("could not get magic block from MPT", zap.Error(err))
		return false
	}
	magicBlock := block.NewMagicBlock()
	err = magicBlock.Decode(magicBlockBytes.Encode())
	if err != nil {
		Logger.Error("could not decode magic block from MPT", zap.Error(err))
		return false
	}
	balances.SetMagicBlock(magicBlock)
	return true
}

func getFunctionName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}
