package psmdb

import (
	"context"
	"github.com/pkg/errors"
	mgo "go.mongodb.org/mongo-driver/mongo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/percona/percona-server-mongodb-operator/pkg/apis/psmdb/v1"
	"github.com/percona/percona-server-mongodb-operator/pkg/psmdb/mongo"
	"github.com/percona/percona-server-mongodb-operator/pkg/psmdb/tls"
)

type Credentials struct {
	Username string
	Password string
}

func MongoClient(k8sclient client.Client, cr *api.PerconaServerMongoDB, rs api.ReplsetSpec, c Credentials) (*mgo.Client, error) {
	pods, err := GetRSPods(k8sclient, cr, rs.Name)
	if err != nil {
		return nil, errors.Wrapf(err, "get pods list for replset %s", rs.Name)
	}

	rsAddrs, err := GetReplsetAddrs(k8sclient, cr, rs.Name, rs.Expose.Enabled, pods.Items)
	if err != nil {
		return nil, errors.Wrap(err, "get replset addr")
	}

	conf := &mongo.Config{
		ReplSetName: rs.Name,
		Hosts:       rsAddrs,
		Username:    c.Username,
		Password:    c.Password,
	}

	if !cr.Spec.UnsafeConf {
		tlsCfg, err := tls.Config(k8sclient, cr)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get TLS config")
		}

		conf.TLSConf = &tlsCfg
	}

	return mongo.Dial(conf)
}

func MongosClient(k8sclient client.Client, cr *api.PerconaServerMongoDB, c Credentials) (*mgo.Client, error) {
	list := corev1.PodList{}
	err := k8sclient.List(context.TODO(),
		&list,
		&client.ListOptions{
			Namespace: cr.Namespace,
			LabelSelector: labels.SelectorFromSet(map[string]string{
				"app.kubernetes.io/name":       "percona-server-mongodb",
				"app.kubernetes.io/instance":   cr.Name,
				"app.kubernetes.io/component":  "mongos",
				"app.kubernetes.io/managed-by": "percona-server-mongodb-operator",
				"app.kubernetes.io/part-of":    "percona-server-mongodb",
			}),
		},
	)
	if err != nil {
		return nil, errors.Wrap(err, "list mongos pods")
	}
	hosts, err := GetMongosAddrs(k8sclient, cr, list.Items)
	if err != nil {
		return nil, errors.Wrap(err, "get mongos addrs")
	}
	conf := mongo.Config{
		Hosts:    hosts,
		Username: c.Username,
		Password: c.Password,
	}

	if !cr.Spec.UnsafeConf {
		tlsCfg, err := tls.Config(k8sclient, cr)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get TLS config")
		}

		conf.TLSConf = &tlsCfg
	}

	return mongo.Dial(&conf)
}
