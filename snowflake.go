package snowflake

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"
)

type Algorithm struct {
	nodeId    uint64
	startTime time.Time
	// bits
	nodeBits     uint8
	sequenceBits uint8 // sequence最多
	// 位移长度
	nodeMoveLength      uint8
	timestampMoveLength uint8
	// 最大值
	maxNode     uint32 // node最多10bit
	maxSequence uint32 // sequence最多12bit
}

const (
	// 1 bit reserved | 41 bit timestamp | 10 bit node | 12 bit sequence
	timestampBits uint8  = 41
	maxTimestamp  uint64 = 1<<timestampBits - 1
	// 缺省的node bits和sequence bits
	defaultNodeBits     uint8 = 3 // node bits支持2^3=8个节点
	defaultSequenceBits uint8 = 7 // sequence bits同时一个node同一时间最多生成128个sequence
	// 缺省的twitter算法的epoch
	defaultEpoc = int64(1288834974657)
)

var (
	// 转换成time.Time,对应于2010年11月4日 01:42:54.657 UTC
	defaultStartTime = time.Unix(defaultEpoc/1000, (defaultEpoc%1000)*1e6)
	lastTime         int64
	lastSeq          uint32
)

func New(nodeId uint64, options ...Option) (*Algorithm, error) {
	a := &Algorithm{
		startTime:    defaultStartTime,
		nodeBits:     defaultNodeBits,
		sequenceBits: defaultSequenceBits,
	}

	for _, apply := range options {
		err := apply(a)
		if err != nil {
			return nil, err
		}
	}

	// 在 JavaScript 中，这是能够被安全且准确表示的最大整数为2<<53-1
	// 这里强制检查node bits + sequence bits不超过63-41=12
	if a.nodeBits+a.sequenceBits > 12 {
		return nil, errors.New("the node bits and sequence bits cannot be greater than 12")
	}

	// 计算max值
	a.maxNode = 1<<a.nodeBits - 1
	a.maxSequence = 1<<a.sequenceBits - 1

	// 计算位移值
	a.nodeMoveLength = a.sequenceBits
	a.timestampMoveLength = a.sequenceBits + a.nodeBits

	if err := a.setupNodeId(nodeId); err != nil {
		return nil, err
	}

	return a, nil
}

// NextID generate snowflake id and return an error.
// This function is thread safe.
func (a *Algorithm) NextID() (uint64, error) {
	c := currentMillis()

	seq, err := a.atomicSequenceResolver(c)
	if err != nil {
		return 0, err
	}

	for seq >= a.maxSequence {
		c = waitForNextMillis(c)
		seq, err = a.atomicSequenceResolver(c)
		if err != nil {
			return 0, err
		}
	}

	df := elapsedTime(c, a.startTime)
	if df < 0 || uint64(df) > maxTimestamp {
		return 0, errors.New("the maximum life cycle of the snowflake algorithm is 2^41-1(millis), please check starttime")
	}

	id := uint64(df)<<a.timestampMoveLength | a.nodeId<<a.nodeMoveLength | uint64(seq)
	return id, nil
}

// Parse snowflake id to ID struct.
func (a *Algorithm) Parse(id uint64) ID {
	return ID{
		startTime: a.startTime,
		Sequence:  id & uint64(a.maxSequence),
		Node:      (id & (uint64(a.maxNode) << a.sequenceBits)) >> a.sequenceBits,
		Timestamp: id >> uint64(a.timestampMoveLength),
	}
}

func (a *Algorithm) setupNodeId(nodeId uint64) error {
	if nodeId == 0 {
		return errors.New("invalid node id")
	}

	// 有可能多个服务运行snowflake服务，但defaultNodeBits有限, nodeNumber不能大于nodeMax
	if nodeId > uint64(a.maxNode) {
		return fmt.Errorf("the nodeId cannot be greater than %d", a.maxNode)
	}

	a.nodeId = nodeId
	return nil
}

//--------------------------------------------------------------------
// private function defined.
//--------------------------------------------------------------------

func waitForNextMillis(last int64) int64 {
	now := currentMillis()
	for now == last {
		now = currentMillis()
	}
	return now
}

func elapsedTime(noms int64, t time.Time) int64 {
	return noms - t.UTC().UnixNano()/1e6
}

// currentMillis get current millisecond.
func currentMillis() int64 {
	return time.Now().UTC().UnixNano() / 1e6
}

// When you want to use the snowflake algorithm to generate unique ID, You must ensure: The sequence-number generated in the same millisecond of the same node is unique.
// Based on this, we create this interface provide following resolver:
// atomicSequenceResolver define as atomic sequence resolver, base on standard sync/atomic.
func (a *Algorithm) atomicSequenceResolver(ms int64) (uint32, error) {
	var last int64
	var seq, localSeq uint32

	for {
		last = atomic.LoadInt64(&lastTime)
		localSeq = atomic.LoadUint32(&lastSeq)
		if last > ms {
			return a.maxSequence, nil
		}

		if last == ms {
			seq = a.maxSequence & (localSeq + 1)
			if seq == 0 {
				return a.maxSequence, nil
			}
		}

		if atomic.CompareAndSwapInt64(&lastTime, last, ms) && atomic.CompareAndSwapUint32(&lastSeq, localSeq, seq) {
			return seq, nil
		}
	}
}
