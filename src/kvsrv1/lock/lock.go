package lock

import (
	"time"

	"6.5840/kvsrv1/rpc"
	kvtest "6.5840/kvtest1"
)

type Lock struct {
	// IKVClerk is a go interface for k/v clerks: the interface hides
	// the specific Clerk type of ck but promises that ck supports
	// Put and Get.  The tester passes the clerk in when calling
	// MakeLock().
	ck kvtest.IKVClerk
	// You may add code here
	key      string
	clientID string
}

// The tester calls MakeLock() and passes in a k/v clerk; your code can
// perform a Put or Get by calling lk.ck.Put() or lk.ck.Get().
//
// Use l as the key to store the "lock state" (you would have to decide
// precisely what the lock state is).
func MakeLock(ck kvtest.IKVClerk, l string) *Lock {
	lk := &Lock{ck: ck}
	// You may add code here
	lk.key = l
	lk.clientID = kvtest.RandValue(8)
	return lk
}

func (lk *Lock) Acquire() {
	// Your code here
	for {
		// check whether the lock existed and has hold by other
		val, ver, err := lk.ck.Get(lk.key)

		if err == rpc.ErrNoKey {
			// lock directly if no one hold
			err = lk.ck.Put(lk.key, lk.clientID, 0)
			if err == rpc.OK {
				// successful grap the lock
				return
			}

			// code enter here means:
			// - the lock grap by other during the process between get and put
			// - return rpc.ErrMaybe, that means we already grap the lock
			// To make sure we indeed acquire the lock, keep retrying
			continue
		} else if err == rpc.OK {
			// Lock grabbed, check whether the lock grab by ourself
			if val == lk.clientID {
				// we acquire the lock
				return
			} else if val == "" {
				// lock exist, but no one hold
				err = lk.ck.Put(lk.key, lk.clientID, ver)
				if err == rpc.OK {
					// we acquire the lock
					return
				}
			} else {
				// lock grap by other, wait 10 million second & retry
				time.Sleep(time.Millisecond * 10)
				continue
			}
		}
	}
}

func (lk *Lock) Release() {
	// Your code here
	for {
		// check whether the lock existed and has hold by other
		val, ver, err := lk.ck.Get(lk.key)

		if err == rpc.ErrNoKey {
			// the lock is not exist
			return
		} else if err == rpc.OK {
			if val == lk.clientID {
				// we grab the lock, release it
				err = lk.ck.Put(lk.key, "", ver)
				if err == rpc.OK {
					// successful released by us
					return
				} else if err == rpc.ErrVersion {
					// version number is not matched
					continue
				} else if err == rpc.ErrMaybe {
					// rpc.ErrMaybe: not sure the lock been released, try again
					continue
				}
			} else {
				// the lock grab by others
				return
			}
		}
	}
}
