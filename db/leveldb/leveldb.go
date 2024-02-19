package leveldb

import (
	"encoding/json"
	"errors"
	"fmt"
	"igorcrevar/cardano-go-syncer/core"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
)

type LevelDbDatabase struct {
	db *leveldb.DB
}

var (
	txOutputsBucket         = []byte("TXOuts")
	latestBlockPointBucket  = []byte("LatestBlockPoint")
	processedBlocksBucket   = []byte("ProcessedBlocks")
	unprocessedBlocksBucket = []byte("UnprocessedBlocks")
)

var _ core.Database = (*LevelDbDatabase)(nil)

func (lvldb *LevelDbDatabase) Init(filePath string) error {
	db, err := leveldb.OpenFile(filePath, nil)
	if err != nil {
		return fmt.Errorf("could not open db: %v", err)
	}

	lvldb.db = db

	return nil
}

func (bd *LevelDbDatabase) Close() error {
	return bd.db.Close()
}

func (lvldb *LevelDbDatabase) GetLatestBlockPoint() (*core.BlockPoint, error) {
	var result *core.BlockPoint

	bytes, err := lvldb.db.Get(latestBlockPointBucket, nil)
	if err != nil {
		return nil, processNotFoundErr(err)
	}

	if err := json.Unmarshal(bytes, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func (lvldb *LevelDbDatabase) GetTxOutput(txInput core.TxInput) (*core.TxOutput, error) {
	var result *core.TxOutput

	bytes, err := lvldb.db.Get(bucketKey(txOutputsBucket, txInput.Key()), nil)
	if err != nil {
		return nil, processNotFoundErr(err)
	}

	if err := json.Unmarshal(bytes, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func (lvldb *LevelDbDatabase) MarkConfirmedBlockProcessed(block *core.FullBlock) error {
	bytes, err := lvldb.db.Get(bucketKey(unprocessedBlocksBucket, block.Key()), nil)
	if err != nil {
		return err
	}

	batch := new(leveldb.Batch)

	batch.Delete(bucketKey(unprocessedBlocksBucket, block.Key()))
	batch.Put(bucketKey(processedBlocksBucket, block.Key()), bytes)

	return lvldb.db.Write(batch, &opt.WriteOptions{
		NoWriteMerge: false,
		Sync:         true,
	})
}

func (lvldb *LevelDbDatabase) GetUnprocessedConfirmedBlocks() ([]*core.FullBlock, error) {
	var result []*core.FullBlock

	iter := lvldb.db.NewIterator(util.BytesPrefix(unprocessedBlocksBucket), nil)
	defer iter.Release()

	for iter.Next() {
		var block *core.FullBlock

		if err := json.Unmarshal(iter.Value(), &block); err != nil {
			return nil, err
		}

		result = append(result, block)
	}

	return result, iter.Error()
}

func (lvldb *LevelDbDatabase) OpenTx() core.DbTransactionWriter {
	return NewLevelDbTransactionWriter(lvldb.db)
}

func bucketKey(bucket []byte, key []byte) []byte {
	const separator = "_#_"

	outputKey := make([]byte, len(bucket)+len(separator)+len(key))
	copy(outputKey, bucket)
	copy(outputKey[len(bucket):], []byte(separator))
	copy(outputKey[len(bucket)+len(separator):], key)

	return outputKey
}

func processNotFoundErr(err error) error {
	if errors.Is(err, leveldb.ErrNotFound) {
		return nil
	}

	return err
}