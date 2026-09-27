#![forbid(unsafe_code)]

fn main() {
    let _probe = probe_http::HttpProbe::new();
    let _control_plane_role = control_plane_client::CORE_ROLE;
    let _core_role = checker_core::CRATE_ROLE;
}
