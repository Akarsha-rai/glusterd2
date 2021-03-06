package peercommands

import (
	"net/http"

	"github.com/gluster/glusterd2/gdctx"
	"github.com/gluster/glusterd2/peer"
	"github.com/gluster/glusterd2/pkg/api"
	restutils "github.com/gluster/glusterd2/servers/rest/utils"
	"github.com/gluster/glusterd2/store"
	"github.com/gluster/glusterd2/utils"
	"github.com/gluster/glusterd2/volume"

	"github.com/gorilla/mux"
	"github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
)

func deletePeerHandler(w http.ResponseWriter, r *http.Request) {
	peerReq := mux.Vars(r)

	id := peerReq["peerid"]
	if id == "" {
		restutils.SendHTTPError(w, http.StatusBadRequest, "peerid not present in the request", api.ErrCodeDefault)
		return
	}

	// Deleting a peer from the cluster happens as follows,
	// 	- Check if the peer is a member of the cluster
	// 	- Check if the peer can be removed
	//	- Delete the peer info from the store
	//	- Send the Leave request

	logger := log.WithField("peerid", id)
	logger.Debug("received delete peer request")

	// Check whether the member exists
	p, err := peer.GetPeerF(id)
	if err != nil {
		logger.WithError(err).Error("failed to get peer")
		restutils.SendHTTPError(w, http.StatusInternalServerError, "could not validate delete request", api.ErrCodeDefault)
		return
	} else if p == nil {
		logger.Debug("request denied, received request to remove unknown peer")
		restutils.SendHTTPError(w, http.StatusNotFound, "peer not found in cluster", api.ErrCodeDefault)
		return
	}

	// You cannot remove yourself
	if id == gdctx.MyUUID.String() {
		logger.Debug("request denied, received request to delete self from cluster")
		restutils.SendHTTPError(w, http.StatusBadRequest, "removing self is disallowed.", api.ErrCodeDefault)
		return
	}

	// Check if any volumes exist with bricks on this peer
	if exists, err := bricksExist(id); err != nil {
		logger.WithError(err).Error("failed to check if bricks exist on peer")
		restutils.SendHTTPError(w, http.StatusInternalServerError, "could not validate delete request", api.ErrCodeDefault)
		return
	} else if exists {
		logger.Debug("request denied, peer has bricks")
		restutils.SendHTTPError(w, http.StatusForbidden, "cannot delete peer, peer has bricks", api.ErrCodeDefault)
		return
	}

	// Remove the peer details from the store
	if err := peer.DeletePeer(id); err != nil {
		log.WithError(err).WithField("peer", id).Error("failed to remove peer from the store")
		restutils.SendHTTPError(w, http.StatusInternalServerError, err.Error(), api.ErrCodeDefault)
		return
	}

	remotePeerAddress, err := utils.FormRemotePeerAddress(p.Addresses[0])
	if err != nil {
		log.WithError(err).WithField("address", p.Addresses[0]).Error("failed to parse peer address")
		restutils.SendHTTPError(w, http.StatusBadRequest, "failed to parse remote address", api.ErrCodeDefault)
		return
	}

	client, err := getPeerServiceClient(remotePeerAddress)
	if err != nil {
		restutils.SendHTTPError(w, http.StatusInternalServerError, err.Error(), api.ErrCodeDefault)
		return
	}
	defer client.conn.Close()

	// TODO: Need to do a better job of handling failures here. If this fails the
	// peer being removed still thinks it's a part of the cluster, and could
	// potentially still send commands to the cluster
	rsp, err := client.LeaveCluster()
	if err != nil {
		logger.WithError(err).Error("sending Leave request failed")
		restutils.SendHTTPError(w, http.StatusInternalServerError, "failed to send leave cluster request", api.ErrCodeDefault)
		return
	} else if Error(rsp.Err) != ErrNone {
		err = Error(rsp.Err)
		logger.WithError(err).Error("leave request failed")
		restutils.SendHTTPError(w, http.StatusInternalServerError, err.Error(), api.ErrCodeDefault)
		return
	}
	logger.Debug("peer left cluster")

	restutils.SendHTTPResponse(w, http.StatusNoContent, nil)

	// Save updated store endpoints for restarts
	store.Store.UpdateEndpoints()
}

// bricksExist checks if the given peer has any bricks on it
// TODO: Move this to a more appropriate place
func bricksExist(id string) (bool, error) {
	pid := uuid.Parse(id)

	vols, err := volume.GetVolumes()
	if err != nil {
		return true, err
	}

	for _, v := range vols {
		for _, b := range v.Bricks {
			if uuid.Equal(pid, b.NodeID) {
				return true, nil
			}
		}
	}
	return false, nil
}
