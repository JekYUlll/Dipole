const password = os.getenv("MYSQL_ROOT_PASSWORD");
const clusterPrimary = os.getenv("MYSQL_CLUSTER_PRIMARY");
const rejoinHost = os.getenv("MYSQL_REJOIN_HOST");
shell.options.useWizards = false;

shell.connect({ host: clusterPrimary, port: 3306, user: "root", password: password });
const cluster = dba.getCluster("dipole");
cluster.rejoinInstance({ host: rejoinHost, port: 3306, user: "root", password: password });
cluster.status({ extended: 1 });
