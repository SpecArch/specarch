import Observation
import SwiftUI

/// Where the screens are: the path the app's NavigationStack follows, the
/// page sign-in was asked for from, and the message an event left for the
/// page it leads to. The app makes one and puts it in the environment.
@MainActor
@Observable
public final class Router {
    /// The routes pushed on the app's navigation stack.
    public var path: [Route] = []
    /// The page sign-in was asked for from, opened again once it is done.
    public var returnTo: Route?
    /// A message left for a page, shown there once it opens.
    public internal(set) var notice: Notice?

    public init() {}

    /// Opens a page.
    public func go(to route: Route) {
        path.append(route)
    }

    /// Opens a page in place of the one shown.
    public func replace(with route: Route) {
        if path.isEmpty {
            path = [route]
        } else {
            path[path.count - 1] = route
        }
    }

    /// Sends the person to sign in, keeping the page they asked for.
    public func askToSignIn(from route: Route) {
        guard let signIn = Application.signIn else {
            return
        }
        returnTo = route
        replace(with: signIn)
    }

    /// Where a task page's event leads: back to the page sign-in was asked
    /// for from when the event does not keep it, else to the page it names.
    public func follow(_ event: EventSchema, routes: [String: Routes.Make], answer: JSON?) -> Route? {
        guard let navigate = event.navigate else {
            return nil
        }
        if event.keepsReturnTo != true, let back = returnTo {
            returnTo = nil
            return back
        }
        guard let make = routes[navigate] else {
            return nil
        }
        var parameters: [String: String] = [:]
        for (name, from) in event.with ?? [:] {
            parameters[name] = answer?[from]?.text ?? ""
        }
        return make(parameters)
    }

    /// Where a task page's answer that leads to the person's home goes: back
    /// to the page sign-in was asked for from, else the home of the first of
    /// their roles that has one, read from the session again, else the
    /// application's start (nil). The page left is taken off the stack.
    public func goHome(_ store: SessionStore) async -> Route? {
        if let back = returnTo {
            returnTo = nil
            path = [back]
            return back
        }
        await store.read()
        guard let session = store.current, let home = Access.home(Application.homes, session: session) else {
            path = []
            return nil
        }
        path = [home]
        return home
    }

    /// Leaves a message for the page a route opens, or for the page shown.
    public func leave(_ text: String, at route: Route?) {
        notice = Notice(text: text, at: route ?? path.last)
    }

    /// Takes the message left for the page shown, if any.
    public func noticeShown() -> String? {
        guard let notice, notice.at == path.last else {
            return nil
        }
        return notice.text
    }

    /// Forgets the message shown.
    public func forgetNotice() {
        notice = nil
    }
}

/// A message an event left, and the page it is for.
public struct Notice: Sendable, Equatable {
    public var text: String
    public var at: Route?
}

/// How a page's routes are made from the parameters an event or a row gives.
public enum Routes {
    /// Makes a route from its parameters, each by its name in the route.
    public typealias Make = @Sendable ([String: String]) -> Route
}
