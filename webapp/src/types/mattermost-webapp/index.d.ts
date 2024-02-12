export interface PluginRegistry {
    registerPostTypeComponent(typeName: string, component: React.ElementType)
    registerRightHandSidebarComponent(title: string, component: React.Component)
}
