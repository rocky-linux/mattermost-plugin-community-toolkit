import { Action, Store } from "redux";
import { GlobalState } from "@mattermost/types/lib/store";
import { PluginRegistry } from "@/types/mattermost-webapp";


export default class Plugin {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars, @typescript-eslint/no-empty-function
  public async initialize(
    registry: PluginRegistry,
    store: Store<GlobalState, Action<Record<string, unknown>>>
  ) {
    // @see https://developers.mattermost.com/extend/plugins/webapp/reference/
  }
}

