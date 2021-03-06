package market

import (
	"fmt"
	"math"
	"time"

	"github.com/ninjadotorg/SimEcon/common"
	"github.com/ninjadotorg/SimEcon/macro_economy/abstraction"
	"github.com/ninjadotorg/SimEcon/macro_economy/dto"
)

type Market struct {
}

var market *Market

func GetMarketInstance() *Market {
	if market != nil {
		return market
	}
	market = &Market{}
	return market
}

func (m *Market) Buy(
	agentID string,
	orderItemReq *dto.OrderItem,
	st abstraction.Storage,
	am abstraction.AccountManager,
	prod abstraction.Production,
	tr abstraction.Tracker,
) (float64, error) {
	sortedBidsByAssetType := st.GetSortedBidsByAssetType(orderItemReq.AssetType, false, common.ORDER_TIME)

	removingBidAgentIDs := []string{}
	for _, bid := range sortedBidsByAssetType {
		if bid.GetPricePerUnit() > orderItemReq.PricePerUnit {
			continue
		}

		sellerAsset, _ := st.GetAgentAsset(
			bid.GetAgentID(),
			bid.GetAssetType(),
		)
		sellerActualAsset := prod.GetActualAsset(sellerAsset)
		actualBidQty := math.Min(sellerActualAsset.GetQuantity(), bid.GetQuantity())
		if actualBidQty >= orderItemReq.Quantity {
			am.Pay(
				agentID,
				bid.GetAgentID(),
				bid.GetPricePerUnit()*orderItemReq.Quantity,
				common.PRIIC,
				common.COIN,
			)
			bid.SetQuantity(actualBidQty - orderItemReq.Quantity)
			if bid.GetQuantity() == 0 {
				removingBidAgentIDs = append(removingBidAgentIDs, bid.GetAgentID())
			}
			sellerActualAsset.SetQuantity(sellerActualAsset.GetQuantity() - orderItemReq.Quantity)
			st.UpdateAsset(bid.GetAgentID(), sellerActualAsset)
			orderItemReq.Quantity = 0
			break
		}
		am.Pay(
			agentID,
			bid.GetAgentID(),
			bid.GetPricePerUnit()*actualBidQty,
			common.PRIIC,
			common.COIN,
		)
		orderItemReq.Quantity -= actualBidQty
		sellerActualAsset.SetQuantity(sellerActualAsset.GetQuantity() - actualBidQty)
		st.UpdateAsset(bid.GetAgentID(), sellerActualAsset)

		bid.SetQuantity(0)
		removingBidAgentIDs = append(removingBidAgentIDs, bid.GetAgentID())
	}
	// re-update bid list: remove bid with qty = 0 and append new ask if remaning qty > 0
	if len(removingBidAgentIDs) > 0 {
		err := st.RemoveBidsByAgentIDs(removingBidAgentIDs, orderItemReq.AssetType)
		if err != nil {
			return -1, err
		}
	}

	if orderItemReq.Quantity > 0 {
		st.AppendAsk(
			orderItemReq.AssetType,
			orderItemReq.AgentID,
			orderItemReq.Quantity,
			orderItemReq.PricePerUnit,
		)
		fmt.Println("----Ask----")
		fmt.Println("Asset type: ", orderItemReq.AssetType)
		fmt.Println("Agent ID: ", orderItemReq.AgentID)
		fmt.Println("Quantity: ", orderItemReq.Quantity)
		fmt.Println("PricePerUnit: ", orderItemReq.PricePerUnit)
		totalAsks := st.GetTotalAsksByAssetType(orderItemReq.AssetType)
		record := []string{fmt.Sprintf("%d", time.Now().Unix()), fmt.Sprintf("%.1f", totalAsks)}

		err := tr.WriteToCSV(fmt.Sprintf("%s_%d.csv", common.TOTAL_ASKS_FILE, orderItemReq.AssetType), record)
		if err != nil {
			return orderItemReq.Quantity, err
		}
	}

	return orderItemReq.Quantity, nil
}

func (m *Market) Sell(
	agentID string,
	orderItemReq *dto.OrderItem,
	st abstraction.Storage,
	am abstraction.AccountManager,
	prod abstraction.Production,
	tr abstraction.Tracker,
) (float64, error) {
	sortedAsksByAssetType := st.GetSortedAsksByAssetType(orderItemReq.AssetType, false, common.ORDER_TIME)

	removingAskAgentIDs := []string{}
	for _, ask := range sortedAsksByAssetType {
		if ask.GetPricePerUnit() < orderItemReq.PricePerUnit {
			continue
		}

		buyerAsset, _ := st.GetAgentAsset(
			ask.GetAgentID(),
			ask.GetAssetType(),
		)
		buyerActualAsset := prod.GetActualAsset(buyerAsset)

		askBalance := am.GetBalance(ask.GetAgentID(), common.COIN)
		if askBalance < ask.GetPricePerUnit()*math.Min(orderItemReq.Quantity, ask.GetQuantity()) {
			removingAskAgentIDs = append(removingAskAgentIDs, ask.GetAgentID())
			continue
		}

		if ask.GetQuantity() >= orderItemReq.Quantity {
			am.Pay(
				ask.GetAgentID(),
				agentID,
				ask.GetPricePerUnit()*orderItemReq.Quantity,
				common.PRIIC,
				common.COIN,
			)
			ask.SetQuantity(ask.GetQuantity() - orderItemReq.Quantity)
			if ask.GetQuantity() == 0 {
				removingAskAgentIDs = append(removingAskAgentIDs, ask.GetAgentID())
			}

			buyerActualAsset.SetQuantity(buyerActualAsset.GetQuantity() + orderItemReq.Quantity)
			st.UpdateAsset(ask.GetAgentID(), buyerActualAsset)

			orderItemReq.Quantity = 0
			break
		}
		am.Pay(
			ask.GetAgentID(),
			agentID,
			ask.GetPricePerUnit()*ask.GetQuantity(),
			common.PRIIC,
			common.COIN,
		)

		buyerActualAsset.SetQuantity(buyerActualAsset.GetQuantity() + ask.GetQuantity())
		st.UpdateAsset(ask.GetAgentID(), buyerActualAsset)

		orderItemReq.Quantity -= ask.GetQuantity()
		ask.SetQuantity(0)
		removingAskAgentIDs = append(removingAskAgentIDs, ask.GetAgentID())
	}
	// re-update ask list: remove ask with qty = 0 and append new ask if remaning qty > 0
	if len(removingAskAgentIDs) > 0 {
		err := st.RemoveAsksByAgentIDs(removingAskAgentIDs, orderItemReq.AssetType)
		if err != nil {
			return -1, err
		}
	}

	if orderItemReq.Quantity > 0 {
		st.AppendBid(
			orderItemReq.AssetType,
			orderItemReq.AgentID,
			orderItemReq.Quantity,
			orderItemReq.PricePerUnit,
		)
		fmt.Println("----Bid----")
		fmt.Println("Asset type: ", orderItemReq.AssetType)
		fmt.Println("Agent ID: ", orderItemReq.AgentID)
		fmt.Println("Quantity: ", orderItemReq.Quantity)
		fmt.Println("PricePerUnit: ", orderItemReq.PricePerUnit)
		totalBids := st.GetTotalBidsByAssetType(orderItemReq.AssetType)
		record := []string{fmt.Sprintf("%d", time.Now().Unix()), fmt.Sprintf("%.1f", totalBids)}
		fmt.Println("Record: ", record)
		err := tr.WriteToCSV(fmt.Sprintf("%s_%d.csv", common.TOTAL_BIDS_FILE, orderItemReq.AssetType), record)
		if err != nil {
			return orderItemReq.Quantity, err
		}
	}

	return orderItemReq.Quantity, nil
}

func (m *Market) SellTokens(
	orderItemReq *dto.OrderItem,
	st abstraction.Storage,
	am abstraction.AccountManager,
	tr abstraction.Tracker,
) (float64, error) {
	var exchangeTokenType uint = common.COIN
	if orderItemReq.AssetType == common.COIN {
		exchangeTokenType = common.BOND
	}
	sortedAsksByTokenType := st.GetSortedAsksByAssetType(orderItemReq.AssetType, true, common.PRICE_PER_UINT)
	removingAskAgentIDs := []string{}
	for _, ask := range sortedAsksByTokenType {
		if orderItemReq.Quantity <= ask.GetQuantity() {
			amt := orderItemReq.Quantity * ask.GetPricePerUnit()
			am.PayFrom(ask.GetAgentID(), amt, exchangeTokenType)
			am.PayTo(ask.GetAgentID(), orderItemReq.Quantity, common.SECIC, orderItemReq.AssetType)
			ask.SetQuantity(ask.GetQuantity() - orderItemReq.Quantity)
			if ask.GetQuantity() == 0 {
				removingAskAgentIDs = append(removingAskAgentIDs, ask.GetAgentID())
			}
			orderItemReq.Quantity = 0
			break
		}
		amt := ask.GetQuantity() * ask.GetPricePerUnit()
		am.PayFrom(ask.GetAgentID(), amt, exchangeTokenType)
		am.PayTo(ask.GetAgentID(), ask.GetQuantity(), common.SECIC, orderItemReq.AssetType)
		orderItemReq.Quantity -= ask.GetQuantity()
		ask.SetQuantity(0)
		removingAskAgentIDs = append(removingAskAgentIDs, ask.GetAgentID())
	}
	// re-update ask list: remove ask with qty = 0 and append new ask if remaning qty > 0
	if len(removingAskAgentIDs) > 0 {
		err := st.RemoveAsksByAgentIDs(removingAskAgentIDs, orderItemReq.AssetType)
		if err != nil {
			return -1, err
		}
	}

	if orderItemReq.Quantity > 0 {
		st.AppendBid(
			orderItemReq.AssetType,
			common.DEFAULT_AGENT_ID,
			orderItemReq.Quantity,
			-1,
		)
		fmt.Println("----Bid----")
		fmt.Println("Asset type: ", orderItemReq.AssetType)
		fmt.Println("Quantity: ", orderItemReq.Quantity)
		totalBids := st.GetTotalBidsByAssetType(orderItemReq.AssetType)
		record := []string{fmt.Sprintf("%d", time.Now().Unix()), fmt.Sprintf("%.1f", totalBids)}
		fmt.Println("Record: ", record)
		err := tr.WriteToCSV(fmt.Sprintf("%s_%d.csv", common.TOTAL_BIDS_FILE, orderItemReq.AssetType), record)
		if err != nil {
			return orderItemReq.Quantity, err
		}
	}
	return orderItemReq.Quantity, nil
}

func (m *Market) BuyTokens(
	buyerID string,
	orderItemReq *dto.OrderItem,
	st abstraction.Storage,
	am abstraction.AccountManager,
	tr abstraction.Tracker,
) (float64, error) {
	var exchangeTokenType uint = common.COIN
	if orderItemReq.AssetType == common.COIN {
		exchangeTokenType = common.BOND
	}
	sortedBidsByTokenType := st.GetSortedBidsByAssetType(orderItemReq.AssetType, true, common.PRICE_PER_UINT)

	if len(sortedBidsByTokenType) == 0 {
		st.AppendAsk(
			orderItemReq.AssetType,
			orderItemReq.AgentID,
			orderItemReq.Quantity,
			orderItemReq.PricePerUnit,
		)
		return orderItemReq.Quantity, nil
	}

	for _, bid := range sortedBidsByTokenType { // because order token in
		bidQty := bid.GetQuantity()
		qty := math.Min(bidQty, orderItemReq.Quantity)
		amt := qty * orderItemReq.PricePerUnit
		am.PayFrom(buyerID, amt, exchangeTokenType)
		am.PayTo(buyerID, qty, common.SECIC, orderItemReq.AssetType)

		if bidQty > orderItemReq.Quantity {
			bid.SetQuantity(bidQty - orderItemReq.Quantity)
			return 0, nil
		}
		// remove current bid
		err := st.RemoveBidsByAgentIDs([]string{common.DEFAULT_AGENT_ID}, orderItemReq.AssetType)
		if err != nil {
			return -1, err
		}
		// append order item to asks
		orderItemReq.Quantity -= bidQty
		if orderItemReq.Quantity > 0 {
			st.AppendAsk(
				orderItemReq.AssetType,
				orderItemReq.AgentID,
				orderItemReq.Quantity,
				orderItemReq.PricePerUnit,
			)
			return orderItemReq.Quantity, nil
		}
		return 0, nil
	}
	return 0, nil
}
