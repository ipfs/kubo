package namesys

import (
	"bytes"
	"errors"

	pb "github.com/ipfs/go-ipfs/namesys/pb"

	u "gx/ipfs/QmNiJuT8Ja3hMVpBHXv3Q6dwmperaQ6JjLtpMQgMCD7xvx/go-ipfs-util"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
)

// IpnsSelectorFunc selects the best record by checking which has the highest
// sequence number and latest EOL
func IpnsSelectorFunc(k string, vals [][]byte) (int, error) {
	var recs []*pb.IpnsEntry
	for _, v := range vals {
		e := new(pb.IpnsEntry)
		err := proto.Unmarshal(v, e)
		if err == nil {
			recs = append(recs, e)
		} else {
			recs = append(recs, nil)
		}
	}

	return selectRecord(recs, vals)
}

func selectRecord(recs []*pb.IpnsEntry, vals [][]byte) (int, error) {
	var bestSeq uint64
	besti := -1

	for i, r := range recs {
		if r == nil || r.GetSequence() < bestSeq {
			continue
		}
		rt, err := u.ParseRFC3339(string(r.GetValidity()))
		if err != nil {
			log.Errorf("failed to parse ipns record EOL %s", r.GetValidity())
			continue
		}

		if besti == -1 || r.GetSequence() > bestSeq {
			bestSeq = r.GetSequence()
			besti = i
		} else if r.GetSequence() == bestSeq {
			bestt, _ := u.ParseRFC3339(string(recs[besti].GetValidity()))
			if rt.After(bestt) {
				besti = i
			} else if rt == bestt {
				if bytes.Compare(vals[i], vals[besti]) > 0 {
					besti = i
				}
			}
		}
	}
	if besti == -1 {
		return 0, errors.New("no usable records in given set")
	}

	return besti, nil
}
