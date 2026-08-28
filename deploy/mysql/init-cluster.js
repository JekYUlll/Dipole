const password = os.getenv("MYSQL_ROOT_PASSWORD");
shell.options.useWizards = false;
const primary = { host: "mysql-1", port: 3306, user: "root", password: password };
const members = [
  { host: "mysql-2", port: 3306, user: "root", password: password },
  { host: "mysql-3", port: 3306, user: "root", password: password },
];

for (const instance of [primary, ...members]) {
  dba.configureInstance(instance, { restart: false });
}

shell.connect(primary);
let cluster;
try {
  cluster = dba.getCluster("dipole");
} catch (error) {
  cluster = dba.createCluster("dipole", {
    communicationStack: "MYSQL",
    gtidSetIsComplete: true,
  });
}

for (const instance of members) {
  try {
    cluster.addInstance(instance, { recoveryMethod: "clone" });
  } catch (error) {
    if (!String(error).includes("already part of")) {
      throw error;
    }
  }
}

cluster.status({ extended: 1 });
