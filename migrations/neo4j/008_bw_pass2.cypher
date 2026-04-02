// BW Pass 2 support: BWCluster nodes for cross-file entity grouping
CREATE INDEX bw_cluster_name IF NOT EXISTS FOR (n:BWCluster) ON (n.name);
