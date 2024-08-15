function toggleMenu() {
    const nav = document.getElementById("side-nav");
    nav.classList.toggle("open");

    const main = document.querySelector("main");
    main.classList.toggle("shifted");
}
