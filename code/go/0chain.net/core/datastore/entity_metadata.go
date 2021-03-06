package datastore

type InstanceProvider func() Entity

var entityMetadataMap = make(map[string]EntityMetadata)

/*RegisterEntityProvider - keep track of a list of entity providers. An entity can be registered with multiple names
* as long as two entities don't use the same name
 */
func RegisterEntityMetadata(entityName string, entityMetadata EntityMetadata) {
	entityMetadataMap[entityName] = entityMetadata
}

/*GetEntityMetadata - return an instance of the entity */
func GetEntityMetadata(entityName string) EntityMetadata {
	return entityMetadataMap[entityName]
}

/*GetEntity - return an instance of the entity */
func GetEntity(entityName string) Entity {
	return GetEntityMetadata(entityName).Instance()
}

type EntityMetadata interface {
	GetName() string
	GetDB() string
	Instance() Entity
	GetStore() Store
	GetIDColumnName() string
}

type EntityMetadataImpl struct {
	Name         string
	DB           string
	Store        Store
	Provider     InstanceProvider
	IDColumnName string
}

func MetadataProvider() *EntityMetadataImpl {
	em := EntityMetadataImpl{IDColumnName: "id"}
	return &em
}

func (em *EntityMetadataImpl) GetName() string {
	return em.Name
}

func (em *EntityMetadataImpl) GetDB() string {
	return em.DB
}

func (em *EntityMetadataImpl) Instance() Entity {
	return em.Provider()
}

func (em *EntityMetadataImpl) GetStore() Store {
	return em.Store
}

func (em *EntityMetadataImpl) GetIDColumnName() string {
	return em.IDColumnName
}
