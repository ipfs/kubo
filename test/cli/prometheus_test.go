package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/ipfs/kubo/test/cli/testutils"
	"github.com/prometheus/common/expfmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var metricsRcmgrFamilies = map[string]bool{
	"libp2p_rcmgr_memory_allocations_allowed_total": true,
	"libp2p_rcmgr_memory_allocations_blocked_total": true,
	"libp2p_rcmgr_peer_blocked_total":               true,
	"libp2p_rcmgr_peers_allowed_total":              true,
}

var metricsDefaultFamilies = map[string]bool{
	"flatfs_datastore_batchcommit_errors_total":            true,
	"flatfs_datastore_batchcommit_latency_seconds":         true,
	"flatfs_datastore_batchcommit_total":                   true,
	"flatfs_datastore_batchdelete_errors_total":            true,
	"flatfs_datastore_batchdelete_latency_seconds":         true,
	"flatfs_datastore_batchdelete_total":                   true,
	"flatfs_datastore_batchput_errors_total":               true,
	"flatfs_datastore_batchput_latency_seconds":            true,
	"flatfs_datastore_batchput_size_bytes":                 true,
	"flatfs_datastore_batchput_total":                      true,
	"flatfs_datastore_check_errors_total":                  true,
	"flatfs_datastore_check_latency_seconds":               true,
	"flatfs_datastore_check_total":                         true,
	"flatfs_datastore_delete_errors_total":                 true,
	"flatfs_datastore_delete_latency_seconds":              true,
	"flatfs_datastore_delete_total":                        true,
	"flatfs_datastore_du_errors_total":                     true,
	"flatfs_datastore_du_latency_seconds":                  true,
	"flatfs_datastore_du_total":                            true,
	"flatfs_datastore_gc_errors_total":                     true,
	"flatfs_datastore_gc_latency_seconds":                  true,
	"flatfs_datastore_gc_total":                            true,
	"flatfs_datastore_get_errors_total":                    true,
	"flatfs_datastore_get_latency_seconds":                 true,
	"flatfs_datastore_get_size_bytes":                      true,
	"flatfs_datastore_get_total":                           true,
	"flatfs_datastore_getsize_errors_total":                true,
	"flatfs_datastore_getsize_latency_seconds":             true,
	"flatfs_datastore_getsize_total":                       true,
	"flatfs_datastore_has_errors_total":                    true,
	"flatfs_datastore_has_latency_seconds":                 true,
	"flatfs_datastore_has_total":                           true,
	"flatfs_datastore_put_errors_total":                    true,
	"flatfs_datastore_put_latency_seconds":                 true,
	"flatfs_datastore_put_size_bytes":                      true,
	"flatfs_datastore_put_total":                           true,
	"flatfs_datastore_query_errors_total":                  true,
	"flatfs_datastore_query_latency_seconds":               true,
	"flatfs_datastore_query_total":                         true,
	"flatfs_datastore_scrub_errors_total":                  true,
	"flatfs_datastore_scrub_latency_seconds":               true,
	"flatfs_datastore_scrub_total":                         true,
	"flatfs_datastore_sync_errors_total":                   true,
	"flatfs_datastore_sync_latency_seconds":                true,
	"flatfs_datastore_sync_total":                          true,
	"go_gc_duration_seconds":                               true,
	"go_goroutines":                                        true,
	"go_info":                                              true,
	"go_memstats_alloc_bytes":                              true,
	"go_memstats_alloc_bytes_total":                        true,
	"go_memstats_buck_hash_sys_bytes":                      true,
	"go_memstats_frees_total":                              true,
	"go_memstats_gc_sys_bytes":                             true,
	"go_memstats_heap_alloc_bytes":                         true,
	"go_memstats_heap_idle_bytes":                          true,
	"go_memstats_heap_inuse_bytes":                         true,
	"go_memstats_heap_objects":                             true,
	"go_memstats_heap_released_bytes":                      true,
	"go_memstats_heap_sys_bytes":                           true,
	"go_memstats_last_gc_time_seconds":                     true,
	"go_memstats_lookups_total":                            true,
	"go_memstats_mallocs_total":                            true,
	"go_memstats_mcache_inuse_bytes":                       true,
	"go_memstats_mcache_sys_bytes":                         true,
	"go_memstats_mspan_inuse_bytes":                        true,
	"go_memstats_mspan_sys_bytes":                          true,
	"go_memstats_next_gc_bytes":                            true,
	"go_memstats_other_sys_bytes":                          true,
	"go_memstats_stack_inuse_bytes":                        true,
	"go_memstats_stack_sys_bytes":                          true,
	"go_memstats_sys_bytes":                                true,
	"go_threads":                                           true,
	"ipfs_bitswap_active_block_tasks":                      true,
	"ipfs_bitswap_active_tasks":                            true,
	"ipfs_bitswap_pending_block_tasks":                     true,
	"ipfs_bitswap_pending_tasks":                           true,
	"ipfs_bitswap_recv_all_blocks_bytes":                   true,
	"ipfs_bitswap_recv_dup_blocks_bytes":                   true,
	"ipfs_bitswap_send_times":                              true,
	"ipfs_bitswap_sent_all_blocks_bytes":                   true,
	"ipfs_bitswap_want_blocks_total":                       true,
	"ipfs_bitswap_wantlist_total":                          true,
	"ipfs_bs_cache_arc_hits_total":                         true,
	"ipfs_bs_cache_arc_total":                              true,
	"ipfs_fsrepo_datastore_batchcommit_errors_total":       true,
	"ipfs_fsrepo_datastore_batchcommit_latency_seconds":    true,
	"ipfs_fsrepo_datastore_batchcommit_total":              true,
	"ipfs_fsrepo_datastore_batchdelete_errors_total":       true,
	"ipfs_fsrepo_datastore_batchdelete_latency_seconds":    true,
	"ipfs_fsrepo_datastore_batchdelete_total":              true,
	"ipfs_fsrepo_datastore_batchput_errors_total":          true,
	"ipfs_fsrepo_datastore_batchput_latency_seconds":       true,
	"ipfs_fsrepo_datastore_batchput_size_bytes":            true,
	"ipfs_fsrepo_datastore_batchput_total":                 true,
	"ipfs_fsrepo_datastore_check_errors_total":             true,
	"ipfs_fsrepo_datastore_check_latency_seconds":          true,
	"ipfs_fsrepo_datastore_check_total":                    true,
	"ipfs_fsrepo_datastore_delete_errors_total":            true,
	"ipfs_fsrepo_datastore_delete_latency_seconds":         true,
	"ipfs_fsrepo_datastore_delete_total":                   true,
	"ipfs_fsrepo_datastore_du_errors_total":                true,
	"ipfs_fsrepo_datastore_du_latency_seconds":             true,
	"ipfs_fsrepo_datastore_du_total":                       true,
	"ipfs_fsrepo_datastore_gc_errors_total":                true,
	"ipfs_fsrepo_datastore_gc_latency_seconds":             true,
	"ipfs_fsrepo_datastore_gc_total":                       true,
	"ipfs_fsrepo_datastore_get_errors_total":               true,
	"ipfs_fsrepo_datastore_get_latency_seconds":            true,
	"ipfs_fsrepo_datastore_get_size_bytes":                 true,
	"ipfs_fsrepo_datastore_get_total":                      true,
	"ipfs_fsrepo_datastore_getsize_errors_total":           true,
	"ipfs_fsrepo_datastore_getsize_latency_seconds":        true,
	"ipfs_fsrepo_datastore_getsize_total":                  true,
	"ipfs_fsrepo_datastore_has_errors_total":               true,
	"ipfs_fsrepo_datastore_has_latency_seconds":            true,
	"ipfs_fsrepo_datastore_has_total":                      true,
	"ipfs_fsrepo_datastore_put_errors_total":               true,
	"ipfs_fsrepo_datastore_put_latency_seconds":            true,
	"ipfs_fsrepo_datastore_put_size_bytes":                 true,
	"ipfs_fsrepo_datastore_put_total":                      true,
	"ipfs_fsrepo_datastore_query_errors_total":             true,
	"ipfs_fsrepo_datastore_query_latency_seconds":          true,
	"ipfs_fsrepo_datastore_query_total":                    true,
	"ipfs_fsrepo_datastore_scrub_errors_total":             true,
	"ipfs_fsrepo_datastore_scrub_latency_seconds":          true,
	"ipfs_fsrepo_datastore_scrub_total":                    true,
	"ipfs_fsrepo_datastore_sync_errors_total":              true,
	"ipfs_fsrepo_datastore_sync_latency_seconds":           true,
	"ipfs_fsrepo_datastore_sync_total":                     true,
	"ipfs_http_request_duration_seconds":                   true,
	"ipfs_http_request_size_bytes":                         true,
	"ipfs_http_requests_total":                             true,
	"ipfs_http_response_size_bytes":                        true,
	"ipfs_info":                                            true,
	"leveldb_datastore_batchcommit_errors_total":           true,
	"leveldb_datastore_batchcommit_latency_seconds":        true,
	"leveldb_datastore_batchcommit_total":                  true,
	"leveldb_datastore_batchdelete_errors_total":           true,
	"leveldb_datastore_batchdelete_latency_seconds":        true,
	"leveldb_datastore_batchdelete_total":                  true,
	"leveldb_datastore_batchput_errors_total":              true,
	"leveldb_datastore_batchput_latency_seconds":           true,
	"leveldb_datastore_batchput_size_bytes":                true,
	"leveldb_datastore_batchput_total":                     true,
	"leveldb_datastore_check_errors_total":                 true,
	"leveldb_datastore_check_latency_seconds":              true,
	"leveldb_datastore_check_total":                        true,
	"leveldb_datastore_delete_errors_total":                true,
	"leveldb_datastore_delete_latency_seconds":             true,
	"leveldb_datastore_delete_total":                       true,
	"leveldb_datastore_du_errors_total":                    true,
	"leveldb_datastore_du_latency_seconds":                 true,
	"leveldb_datastore_du_total":                           true,
	"leveldb_datastore_gc_errors_total":                    true,
	"leveldb_datastore_gc_latency_seconds":                 true,
	"leveldb_datastore_gc_total":                           true,
	"leveldb_datastore_get_errors_total":                   true,
	"leveldb_datastore_get_latency_seconds":                true,
	"leveldb_datastore_get_size_bytes":                     true,
	"leveldb_datastore_get_total":                          true,
	"leveldb_datastore_getsize_errors_total":               true,
	"leveldb_datastore_getsize_latency_seconds":            true,
	"leveldb_datastore_getsize_total":                      true,
	"leveldb_datastore_has_errors_total":                   true,
	"leveldb_datastore_has_latency_seconds":                true,
	"leveldb_datastore_has_total":                          true,
	"leveldb_datastore_put_errors_total":                   true,
	"leveldb_datastore_put_latency_seconds":                true,
	"leveldb_datastore_put_size_bytes":                     true,
	"leveldb_datastore_put_total":                          true,
	"leveldb_datastore_query_errors_total":                 true,
	"leveldb_datastore_query_latency_seconds":              true,
	"leveldb_datastore_query_total":                        true,
	"leveldb_datastore_scrub_errors_total":                 true,
	"leveldb_datastore_scrub_latency_seconds":              true,
	"leveldb_datastore_scrub_total":                        true,
	"leveldb_datastore_sync_errors_total":                  true,
	"leveldb_datastore_sync_latency_seconds":               true,
	"leveldb_datastore_sync_total":                         true,
	"libp2p_autonat_next_probe_timestamp":                  true,
	"libp2p_autonat_reachability_status":                   true,
	"libp2p_autonat_reachability_status_confidence":        true,
	"libp2p_autorelay_candidate_loop_state":                true,
	"libp2p_autorelay_candidates_circuit_v2_support_total": true,
	"libp2p_autorelay_desired_reservations":                true,
	"libp2p_autorelay_relay_addresses_count":               true,
	"libp2p_autorelay_relay_addresses_updated_total":       true,
	"libp2p_autorelay_reservation_requests_outcome_total":  true,
	"libp2p_autorelay_reservations_closed_total":           true,
	"libp2p_autorelay_reservations_opened_total":           true,
	"libp2p_autorelay_status":                              true,
	"libp2p_eventbus_events_emitted_total":                 true,
	"libp2p_eventbus_subscriber_event_queued":              true,
	"libp2p_eventbus_subscriber_queue_full":                true,
	"libp2p_eventbus_subscriber_queue_length":              true,
	"libp2p_eventbus_subscribers_total":                    true,
	"libp2p_identify_addrs_count":                          true,
	"libp2p_identify_addrs_received":                       true,
	"libp2p_identify_identify_pushes_triggered_total":      true,
	"libp2p_identify_protocols_count":                      true,
	"libp2p_identify_protocols_received":                   true,
	"libp2p_relaysvc_connection_duration_seconds":          true,
	"libp2p_relaysvc_data_transferred_bytes_total":         true,
	"libp2p_relaysvc_status":                               true,
}

func TestPrometheusMetrics(t *testing.T) {
	fetchMetricFamilies := func(n *harness.Node) map[string]bool {
		resp := n.APIClient().Get("/debug/metrics/prometheus")
		parser := &expfmt.TextParser{}
		fams, err := parser.TextToMetricFamilies(strings.NewReader(resp.Body))
		require.NoError(t, err)
		names := map[string]bool{}
		for k := range fams {
			names[k] = true
		}
		return names
	}

	assertMetricsEventuallyContainOnly := func(n *harness.Node, m map[string]bool) {
		missing := &[]string{}
		assert.Eventuallyf(t, func() bool {
			*missing = make([]string, 0)
			fams := fetchMetricFamilies(n)
			for fam := range m {
				_, ok := fams[fam]
				if !ok {
					*missing = append(*missing, fam)
				}
			}
			if len(*missing) > 0 {
				return false
			}
			return len(fams) == len(m)
		}, 20*time.Second, 100*time.Millisecond, "expected metrics: %v", missing)
	}

	t.Run("default configuration", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		expectedFams := testutils.SetUnion(metricsDefaultFamilies, metricsRcmgrFamilies)
		assertMetricsEventuallyContainOnly(node, expectedFams)
	})

	t.Run("resource manager disabled, should not contain rcmgr metrics", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Swarm.ResourceMgr.Enabled = config.False
		})
		node.StartDaemon()
		assertMetricsEventuallyContainOnly(node, metricsDefaultFamilies)
	})
}
