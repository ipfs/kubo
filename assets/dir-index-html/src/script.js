document.addEventListener('DOMContentLoaded', () => {
    const listingsPath = [
        {{ range .Listing }}
            '{{.Path}}',
        {{ end }}
    ];

    const init = {
        method: 'GET',
        headers: {'Accept': 'application/vnd.ipfs.cache.local'}
    };

    const notCachedHTML = '<div title="File not cached locally" class="ipfs-_attention">&nbsp;</div>';

    listingsPath.forEach((element, index) => {
        fetch(element, init)
        .then(response => {
            if (response.status === 404)
                document.getElementsByClassName('not-cached-locally').item(index).innerHTML = notCachedHTML;
        });
    });
});
