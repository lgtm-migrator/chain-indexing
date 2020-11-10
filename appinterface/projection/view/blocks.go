package view

import (
	"fmt"

	jsoniter "github.com/json-iterator/go"

	"github.com/crypto-com/chainindex/appinterface/rdb"
	"github.com/crypto-com/chainindex/internal/utctime"
	_ "github.com/crypto-com/chainindex/test/factory"
)

// Block projection view implemented by relational database
type Blocks struct {
	rdb *rdb.Handle
}

func NewBlocks(handle *rdb.Handle) *Blocks {
	return &Blocks{
		handle,
	}
}

func (view *Blocks) Insert(block *Block) error {
	var err error

	var sql string
	sql, _, err = view.rdb.StmtBuilder.Insert(
		"view_blocks",
	).Columns(
		"height",
		"hash",
		"time",
		"app_hash",
		"committed_council_nodes",
		"transaction_count",
	).Values("?", "?", "?", "?", "?", "?").ToSql()
	if err != nil {
		return fmt.Errorf("error building blocks insertion sql: %v: %w", err, rdb.ErrBuildSQLStmt)
	}

	var committedCouncilNodesJSON string
	if committedCouncilNodesJSON, err = jsoniter.MarshalToString(block.CommittedCouncilNodes); err != nil {
		return fmt.Errorf("error JSON marshalling blocks committed council nodes for insertion: %v: %w", err, rdb.ErrBuildSQLStmt)
	}

	result, err := view.rdb.Exec(sql,
		block.Height,
		block.Hash,
		view.rdb.Tton(&block.Time),
		block.AppHash,
		committedCouncilNodesJSON,
		block.TransactionCount,
	)
	if err != nil {
		return fmt.Errorf("error inserting block into the table: %v: %w", err, rdb.ErrWrite)
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("error inserting block into the table: no rows inserted: %w", rdb.ErrWrite)
	}

	return nil
}

func (view *Blocks) FindBy(identity *BlockIdentity) (*Block, error) {
	var err error

	selectStmtBuilder := view.rdb.StmtBuilder.Select(
		"height", "hash", "time", "app_hash", "committed_council_nodes", "transaction_count",
	).From("view_blocks")
	if identity.MaybeHash != nil {
		selectStmtBuilder = selectStmtBuilder.Where("hash = ?", *identity.MaybeHash)
	} else {
		selectStmtBuilder = selectStmtBuilder.Where("height = ?", *identity.MaybeHeight)
	}

	sql, sqlArgs, err := selectStmtBuilder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("error building blocks selection sql: %v: %w", err, rdb.ErrPrepare)
	}

	var block Block
	var committedCouncilNodesJSON *string
	timeReader := view.rdb.NtotReader()
	if err = view.rdb.QueryRow(sql, sqlArgs...).Scan(
		&block.Height,
		&block.Hash,
		timeReader.ScannableArg(),
		&block.AppHash,
		&committedCouncilNodesJSON,
		&block.TransactionCount,
	); err != nil {
		if err == rdb.ErrNoRows {
			return nil, rdb.ErrNoRows
		}
		return nil, fmt.Errorf("error scanning block row: %v: %w", err, rdb.ErrQuery)
	}
	blockTime, err := timeReader.Parse()
	if err != nil {
		return nil, fmt.Errorf("error parsing block time: %v: %w", err, rdb.ErrQuery)
	}
	block.Time = *blockTime

	var committedCouncilNodes []BlockCommittedCouncilNode
	if err = jsoniter.Unmarshal([]byte(*committedCouncilNodesJSON), &committedCouncilNodes); err != nil {
		return nil, fmt.Errorf("error unmarshalling block council nodes JSON: %v: %w", err, rdb.ErrQuery)
	}

	block.CommittedCouncilNodes = committedCouncilNodes

	return &block, nil
}

func (view *Blocks) Count() (int, error) {
	sql, _, err := view.rdb.StmtBuilder.Select("COUNT(1)").From(
		"view_blocks",
	).ToSql()
	if err != nil {
		return 0, fmt.Errorf("error building blocks count selection sql: %v", err)
	}

	result := view.rdb.QueryRow(sql)
	var count int
	if err := result.Scan(&count); err != nil {
		return 0, fmt.Errorf("error scanning blocks count selection query: %v", err)
	}

	return count, nil
}

func NewRdbBlockCommittedCouncilNodeFromRaw(raw *BlockCommittedCouncilNode) *RdbBlockCommittedCouncilNode {
	return &RdbBlockCommittedCouncilNode{
		Address:    raw.Address,
		Time:       raw.Time.UnixNano(),
		Signature:  raw.Signature,
		IsProposer: raw.IsProposer,
	}
}

func (node *RdbBlockCommittedCouncilNode) ToRaw() *BlockCommittedCouncilNode {
	return &BlockCommittedCouncilNode{
		Address:    node.Address,
		Time:       utctime.FromUnixNano(node.Time),
		Signature:  node.Signature,
		IsProposer: node.IsProposer,
	}
}

type Block struct {
	Height                int64                       `fake:"{+int64}"`
	Hash                  string                      `fake:"{blockhash}"`
	Time                  utctime.UTCTime             `fake:"{utctime}"`
	AppHash               string                      `fake:"{apphash}"`
	TransactionCount      int                         `fake:"{number:0,2147483647}"`
	CommittedCouncilNodes []BlockCommittedCouncilNode `fakesize:"3"`
}

type BlockCommittedCouncilNode struct {
	Address    string          `fake:"{validatoraddress}"`
	Time       utctime.UTCTime `fake:"{utctime}"`
	Signature  string          `fake:"{commitsignature}"`
	IsProposer bool            `fake:"{bool}"`
}

type RdbBlockCommittedCouncilNode struct {
	Address    string `json:"address"`
	Time       int64  `json:"time"`
	Signature  string `json:"signature"`
	IsProposer bool   `json:"is_proposer"`
}

type BlockIdentity struct {
	MaybeHeight *int64
	MaybeHash   *string
}