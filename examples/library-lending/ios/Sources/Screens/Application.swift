/// How the screens read the person's session, where they sign in, and
/// the menu; where they call the service is Service.baseURL, which the app
/// sets at launch.
public enum Application {
    public static let session: SessionSchema? = .init(path: "/sessions/current", signedIn: "signedIn", permissions: "permissions")
    public static let signIn: Route? = .signIn
    public static let signInTitle = "sign-in.title"
    public static let menu: [MenuGroup] = []
}
