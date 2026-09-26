#![forbid(unsafe_code)]

fn main() {
    let _probe = probe_http::HttpProbe::new();
    let _control_plane = control_plane_client::ControlPlaneClient::new();
    let _core_role = checker_core::CRATE_ROLE;
}
