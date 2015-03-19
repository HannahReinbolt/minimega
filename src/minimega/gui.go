// Copyright (2012) Sandia Corporation.
// Under the terms of Contract DE-AC04-94AL85000 with Sandia Corporation,
// the U.S. Government retains certain rights in this software.
//Author: Brian Wright

package main

import (
	"fmt"
	"html"
	"minicli"
	log "minilog"
	"net/http"
	"strconv"
	"strings"
)

const (
	GUI_PORT          = 9526
	defaultVNC string = "/opt/minimega/misc/novnc"
	defaultD3  string = "/opt/minimega/misc/d3"
	HTMLFRAME         = `<!DOCTYPE html>
				<head><title>Minimega GUI</title>
				<link rel="stylesheet" type="text/css" href="/gui/d3/nav.css">
				<link rel="stylesheet" type="text/css" href="/gui/d3/jquery.dataTables.css">
				<script type="text/javascript" language="javascript" src="/gui/d3/jquery-1.11.1.min.js"></script>
				<script type="text/javascript" language="javascript" src="/gui/d3/jquery.dataTables.min.js"></script>
				%s
				</head>
				<body>
				<nav><ul>
				  <!--<li><a href="/gui/vnc">Host List</a></li>-->
				  <li><a href="/gui/all">All VMs</a></li>
				  <li><a href="/gui/tile">VM Tile</a></li>
				  <li><a href="/gui/stats">Host Stats</a></li>
				  <li><a href="/gui/errors">VM Errors</a></li>
				  <li><a href="/gui/state">State of Health</a></li>
				  <li><a href="/gui/map">VM Map</a></li>
				 <!-- <li><a href="/gui/graph">Graph</a></li>
				  <li><a href="/gui/terminal/terminal.html">Terminal(concept)</a></li>-->
				</ul></nav>      
				%s
				</body></html>`

	D3MAP = `    
		<div id="container"></div>
		<script src="/gui/d3/d3.min.js"></script>
		<script src="/gui/d3/topojson.v1.min.js"></script>
		<script>
		d3.select(window).on("resize", throttle);
		var zoom = d3.behavior.zoom().scaleExtent([1, 9]).on("zoom", move);
		var width = document.getElementById('container').offsetWidth;
		var height = width / 2;
		var topo,projection,path,svg,g;
		var graticule = d3.geo.graticule();
		var tooltip = d3.select("#container").append("div").attr("class", "tooltip hidden");
		setup(width,height);
		function setup(width,height){
		  projection = d3.geo.mercator().translate([(width/2), (height/2)]).scale( width / 2 / Math.PI);
		  path = d3.geo.path().projection(projection);
		  svg = d3.select("#container").append("svg")
		      .attr("width", width)
		      .attr("height", height)
		      .call(zoom)
		      .on("click", click)
		      .append("g");
		  g = svg.append("g").on("click", click);
		}
		d3.json("/gui/d3/world-topo-min.json", function(error, world) {
		  var countries = topojson.feature(world, world.objects.countries).features;
		  topo = countries;
		  draw(topo);
		});
		function draw(topo) {
		  svg.append("path")
		  svg.append("path").datum(graticule).attr("class", "graticule").attr("d", path);
		  g.append("path")
		   .datum({type: "LineString", coordinates: [[-180, 0], [-90, 0], [0, 0], [90, 0], [180, 0]]})
		   .attr("class", "equator")
		   .attr("d", path);
		  var country = g.selectAll(".country").data(topo);
		  country.enter().insert("path")
		      .attr("class", "country")
		      .attr("d", path)
		      .attr("id", function(d,i) { return d.id; })
		      .attr("title", function(d,i) { return d.properties.name; })
		      .style("fill", function(d, i) { return d.properties.color; });
		  //offsets for tooltips
		  var offsetL = document.getElementById('container').offsetLeft+20;
		  var offsetT = document.getElementById('container').offsetTop+10;
		  //tooltips
		  country.on("mousemove", function(d,i) {
		      var mouse = d3.mouse(svg.node()).map( function(d) { return parseInt(d); } );
		      tooltip.classed("hidden", false)
			     .attr("style", "left:"+(mouse[0]+offsetL)+"px;top:"+(mouse[1]+offsetT)+"px")
			     .html(d.properties.name);
		      })
		      .on("mouseout",  function(d,i) {
			tooltip.classed("hidden", true);
		      }); 
		%s
		}
		function redraw() {
		  width = document.getElementById('container').offsetWidth;
		  height = width / 2;
		  d3.select('svg').remove();
		  setup(width,height);
		  draw(topo);
		}
		function move() {
		  var t = d3.event.translate;
		  var s = d3.event.scale; 
		  zscale = s;
		  var h = height/4;
		  t[0] = Math.min(
		    (width/height)  * (s - 1), 
		    Math.max( width * (1 - s), t[0] )
		  );
		  t[1] = Math.min(
		    h * (s - 1) + h * s, 
		    Math.max(height  * (1 - s) - h * s, t[1])
		  );
		  zoom.translate(t);
		  g.attr("transform", "translate(" + t + ")scale(" + s + ")");
		  //adjust the country hover stroke width based on zoom level
		  d3.selectAll(".country").style("stroke-width", 1.5 / s);
		}
		var throttleTimer;
		function throttle() {
		  window.clearTimeout(throttleTimer);
		    throttleTimer = window.setTimeout(function() {
		      redraw();
		    }, 200);
		}
		//geo translation on mouse click in map
		function click() {
		  var latlon = projection.invert(d3.mouse(this));
		  console.log(latlon);
		}
		//function to add points and text to the map (used in plotting capitals)
		function addpoint(lat,lon,text) {
		  var gpoint = g.append("g").attr("class", "gpoint").attr("xlink:href","http://www.sandia.gov");
		  var x = projection([lat,lon])[0];
		  var y = projection([lat,lon])[1];
		  gpoint.append("svg:circle").attr("cx", x).attr("cy", y).attr("class","point").attr("r", 1.5);
		  if(text.length>0){    //conditional in case a point has no associated text
		    gpoint.append("text").attr("x", x+2).attr("y", y+2).attr("class","text").text(text);
		  }
		}
		</script>
		`
)

var (
	guiRunning bool
)

var guiCLIHandlers = []minicli.Handler{
	{ // gui
		HelpShort: "start the minimega GUI",
		HelpLong: `
Launch the GUI webserver

This command requires access to an installation of novnc. By default minimega
looks in /opt/minimega/misc/novnc. To set a different path, invoke:

	gui novnc <path to novnc>

To start the webserver on a specific port, issue the web command with the port:

	gui 9526

9526 is the default port.`,
		Patterns: []string{
			"gui [port]",
		},
		Call: wrapSimpleCLI(cliGUI),
	},
}

func init() {
	registerHandlers("gui", guiCLIHandlers)
}

func cliGUI(c *minicli.Command) *minicli.Response {
	resp := &minicli.Response{Host: hostname}

	port := fmt.Sprintf(":%v", GUI_PORT)
	if c.StringArgs["port"] != "" {
		// Check if port is an integer
		p, err := strconv.Atoi(c.StringArgs["port"])
		if err != nil {
			resp.Error = fmt.Sprintf("'%v' is not a valid port", c.StringArgs["port"])
			return resp
		}

		port = fmt.Sprintf(":%v", p)
	}
	noVNC := defaultVNC
	d3 := defaultD3
	if c.StringArgs["path"] != "" {
		noVNC = c.StringArgs["path"]
	}

	if guiRunning {
		resp.Error = "GUI is already running"
	} else {
		go guiStart(port, noVNC, d3)
	}

	return resp
}

func guiStart(port, noVNC string, d3 string) {

	//Look at me! I self-discovered myself!
	//miniLocation, oserr := os.Readlink("/proc/" + strconv.Itoa(os.Getpid()) + "/exe")
	//fmt.Println(miniLocation)
	//vncLocation := defaultVNC
	//d3Location := defaultD3
	//if strings.Split(miniLocation, "/")[1] == "tmp" {
	//	fmt.Println("you found tmp")
	//}
	//if oserr == nil {
	//	vncLocation = miniLocation + "/misc/novnc"
	//	x := strings.Split(vncLocation, "/bin/minimega")
	//	d3Location = miniLocation + "/misc/d3"
	//	y := strings.Split(d3Location, "/bin/minimega")
	//	vncLocation = x[0] + x[1]
	//	d3Location = y[0] + y[1]
	//	fmt.Println(d3Location)
	//	fmt.Println(vncLocation)
	//}

	guiRunning = true
	http.Handle("/gui/novnc/", http.StripPrefix("/gui/novnc/", http.FileServer(http.Dir(noVNC))))
	http.Handle("/gui/d3/", http.StripPrefix("/gui/d3/", http.FileServer(http.Dir(d3))))

	http.HandleFunc("/gui/ws/", vncWsHandler)
	http.HandleFunc("/gui/map", guiMapVMs)
	http.HandleFunc("/gui/errors", guiErrorVMs)
	http.HandleFunc("/gui/state", guiState)
	http.HandleFunc("/gui/stats", guiStats)
	http.HandleFunc("/gui/all", guiAllVMs)
	http.HandleFunc("/gui/tile", guiTiler)
	http.HandleFunc("/gui/vnc/", guiVNC)
	http.HandleFunc("/gui/command/", guiCmd)
	http.HandleFunc("/gui/screenshot/", guiScreenshot)
	http.HandleFunc("/gui/", guiHome)
	http.HandleFunc("/", guiHome)

	err := http.ListenAndServe(port, nil)
	if err != nil {
		log.Error("guiStart: %v", err)
		guiRunning = false
	}
}

func guiScreenshot(w http.ResponseWriter, r *http.Request) {
	url := strings.TrimSpace(r.URL.String())
	urlFields := strings.Split(url, "/")

	if len(urlFields) != 4 {
		w.Write([]byte("usage: screenshot/<hostname>_<vm id>.png"))
		return
	}

	fields := strings.Split(urlFields[3], "_")
	if len(fields) != 2 {
		w.Write([]byte("usage: screenshot/<hostname>_<vm id>.png"))
		return
	}

	host := fields[0]
	vmId := strings.TrimSuffix(fields[1], ".png")

	var respChan chan minicli.Responses

	cmdLocal, err := minicli.CompileCommand(fmt.Sprintf("vm screenshot %v", vmId))
	if err != nil {
		// Should never happen
		log.Fatalln(err)
	}

	cmdRemote, err := minicli.CompileCommand(fmt.Sprintf("mesh send %v vm screenshot %v", host, vmId))
	if err != nil {
		// Should never happen
		log.Fatalln(err)
	}

	if host == hostname {
		respChan = runCommand(cmdLocal, false)
	} else {
		respChan = runCommand(cmdRemote, false)
	}

	for resps := range respChan {
		for _, resp := range resps {
			if resp.Error != "" {
				log.Errorln(resp.Error)
				w.Write([]byte(resp.Error))
				continue
			}

			if resp.Data == nil {
				w.Write([]byte("no png data!"))
				continue
			}

			d := resp.Data.([]byte)
			w.Write(d)
		}
	}
}

func guiCmd(w http.ResponseWriter, r *http.Request) {
	url := strings.TrimSpace(r.URL.String())
	fields := strings.Split(url, "/")
	cmd := fields[3]

	if cmd == "start" {
		mmstartcmd, err := minicli.CompileCommand(fmt.Sprintf(`mesh send all vm start all`))
		if err != nil {
			log.Fatalln(err)
		}
		runCommand(mmstartcmd, true)
		mmstartLcmd, err := minicli.CompileCommand(fmt.Sprintf(`vm start all`))
		if err != nil {
			log.Fatalln(err)
		}
		runCommand(mmstartLcmd, true)
	}
	if cmd == "flush" {
		mmflushcmd, err := minicli.CompileCommand(fmt.Sprintf(`mesh send all vm flush`))
		if err != nil {
			log.Fatalln(err)
		}
		runCommand(mmflushcmd, true)
		mmflushLcmd, err := minicli.CompileCommand(fmt.Sprintf(`vm flush`))
		if err != nil {
			log.Fatalln(err)
		}
		runCommand(mmflushLcmd, true)
	}
}

func guiVNC(w http.ResponseWriter, r *http.Request) {
	url := strings.TrimSpace(r.URL.String())
	if !strings.HasSuffix(url, "/") {
		url += "/"
	}
	fields := strings.Split(url, "/")
	fields = fields[1 : len(fields)-1]
	if len(fields) == 4 {
		title := html.EscapeString(fields[2] + ":" + fields[3]) //change to vm NAME
		path := fmt.Sprintf("/gui/novnc/vnc_auto.html?title=%v&path=gui/ws/%v/%v", title, fields[2], fields[3])
		iframeresize := `<script>
                         	var buffer = 20; //scroll bar buffer
			 	var iframe = document.getElementById('vnc');

			 	function pageY(elem) {
    					return elem.offsetParent ? (elem.offsetTop + pageY(elem.offsetParent)) : elem.offsetTop;
				}

				function resizeIframe() {
    					var height = document.documentElement.clientHeight;
    					height -= pageY(document.getElementById('vnc'))+ buffer ;
   					height = (height < 0) ? 0 : height;
    					document.getElementById('vnc').style.height = height + 'px';
				}

				window.onresize = resizeIframe;  
				window.onload = resizeIframe;  
         		   </script>
			  `

		body := fmt.Sprintf(`<iframe id="vnc" width="100%v" src="%v"></iframe>`, "%", path)
		w.Write([]byte(fmt.Sprintf(HTMLFRAME, iframeresize, body)))
	} else {
		http.NotFound(w, r)
	}
}

func guiHome(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(fmt.Sprintf(HTMLFRAME, "", "")))
}

func guiState(w http.ResponseWriter, r *http.Request) {

	mask := `id,name,tags`
	list := getVMinfo(mask)
	vdata := ``
	for _, row := range list {
		if len(row) != 3 {
			log.Fatal("column count mismatch: %v", row)
		}
		id := row[0]
		name := row[1]

		var tracert string
		var ping string
		var app string
		f := strings.Fields(row[2])
		for _, v := range f {
			v = strings.Trim(v, "[]")
			v2 := strings.Split(v, ":")
			if len(v2) != 2 {
				continue
			}
			if strings.Contains(v2[0], "traceroute") {
				tracert = v2[1]
			} else if strings.Contains(v2[0], "ping") {
				ping = v2[1]
			} else if strings.Contains(v2[0], "app") {
				app = v2[1]
			}
		}
		if tracert == "" || ping == "" || app == "" {
			continue
		}
		vdata += fmt.Sprintf(`<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>`, name, id, tracert, ping, app)
	}
	header := `<thead><tr><th>name</th><th>id</th><th>traceroute</th><th>ping</th><th>app</th></thead>`
	tabletype := `<script type="text/javascript" language="javascript" src="/gui/d3/table.js"></script>`
	body := fmt.Sprintf(`<table id="example" class="hover" cellspacing="0"> %s <tbody> %s </tbody></table>`, header, vdata)
	w.Write([]byte(fmt.Sprintf(HTMLFRAME, tabletype, body)))
}

func guiMapVMs(w http.ResponseWriter, r *http.Request) {
	mask := `id,name,tags`
	list := getVMinfo(mask)
	dataformat := `   addpoint(%s,%s,"%s")` + "\n"
	mapdata := ``
	for _, row := range list {
		if len(row) != 3 {
			log.Fatal("column count mismatch: %v", row)
		}
		name := row[1]

		// grab out lat/long
		var lat string
		var long string
		f := strings.Fields(row[2])
		for _, v := range f {
			v = strings.Trim(v, "[]")
			v2 := strings.Split(v, ":")
			if len(v2) != 2 {
				continue
			}
			if strings.Contains(v2[0], "lat") {
				lat = v2[1]
			} else if strings.Contains(v2[0], "long") {
				long = v2[1]
			}
		}
		if lat == "" || long == "" {
			continue
		}
		mapdata += fmt.Sprintf(dataformat, lat, long, name)
	}

	d3body := fmt.Sprintf(D3MAP, mapdata)
	d3head := `<link rel="stylesheet" type="text/css" href="/gui/d3/d3map.css">`
	w.Write([]byte(fmt.Sprintf(HTMLFRAME, d3head, d3body)))
}

func getVMinfo(mask string) [][]string {
	var tabular [][]string

	cmdHost, err := minicli.CompileCommand(fmt.Sprintf(`.columns %s vm info`, mask))
	if err != nil {
		// Should never happen
		log.Fatalln(err)
	}
	respChan := runCommand(cmdHost, false)

	for r := range respChan {
		tabular = append(tabular, r[0].Tabular...)
	}

	cmdHostAll, err := minicli.CompileCommand(fmt.Sprintf(`.columns %s mesh send all vm info`, mask))
	if err != nil {
		// Should never happen
		log.Fatalln(err)
	}
	respChan = runCommand(cmdHostAll, false)

	for r := range respChan {
		for _, resp := range r {
			if len(r) != 0 {
				tabular = append(tabular, resp.Tabular...)
			}
		}
	}

	return tabular
}

func guiStats(w http.ResponseWriter, r *http.Request) {
	stats := []string{}
	cmdhost, err := minicli.CompileCommand("host") //local host stats
	if err != nil {
		// Should never happen
		log.Fatalln(err)
	}
	respHostChan := runCommand(cmdhost, false)
	g := <-respHostChan
	if len(stats) == 0 { //If stats is empty, i need a header
		header := `<thead><tr>`
		for _, h := range g[0].Header {
			header += `<th>` + h + `</th>`
		}
		header += `</tr></thead><tbody>`
		stats = append(stats, header)
	}
	for _, row := range g[0].Tabular { //local host data
		tl := `<tr>`
		for _, entry := range row {
			tl += `<td>` + entry + `</td>`
		}
		tl += `</tr>`
		stats = append(stats, tl)
	}
	cmdhostall, err := minicli.CompileCommand("mesh send all host") //mesh send all host
	respHostAllChan := runCommand(cmdhostall, false)
	for s := range respHostAllChan {
		if len(s) != 0 { //check if there are other hosts
			for _, node := range s {
				for _, row := range node.Tabular { //mesh data
					tl := `<tr>`
					for _, entry := range row {
						tl += `<td>` + entry + `</td>`
					}
					tl += `</tr>`
					stats = append(stats, tl)
				}
			}
		}
	}
	body := fmt.Sprintf(`<table id="example" class="hover" cellspacing="0"> %s </tbody></table>`, strings.Join(stats, "\n"))
	tabletype := `<script type="text/javascript" language="javascript" src="/gui/d3/stats.js"></script>`
	w.Write([]byte(fmt.Sprintf(HTMLFRAME, tabletype, body)))
}

func guiErrorVMs(writer http.ResponseWriter, request *http.Request) {
	var resp chan minicli.Responses
	var respAll chan minicli.Responses
	mask := "id,name,state,memory,vcpus,migrate,disk,snapshot,initrd,kernel,cdrom,append,bridge,tap,mac,ip,ip6,vlan,uuid,cc_active,tags"
	cmdLocal, err := minicli.CompileCommand(".columns " + mask + " vm info")
	if err != nil {
		// Should never happen
		log.Fatalln(err)
	}
	cmdRemote, err := minicli.CompileCommand(fmt.Sprintf(".columns %s mesh send all vm info", mask))
	if err != nil {
		// Should never happen
		log.Fatalln(err)
	}
	resp = runCommand(cmdLocal, false)
	respAll = runCommand(cmdRemote, false)

	info := []string{}
	g := <-resp
	ga := g[0].Header
	if len(info) == 0 {
		header := `<thead><tr>`
		for _, h := range ga {
			header += `<th>` + h + `</th>`
			if h == "id" {
				header += `<th>` + `host` + `</th>`
			}
		}
		header += `</tr></thead><tbody>`
		info = append(info, header)
	}

	r := g[0].Tabular
	for _, r := range r {
		if r[2] == "ERROR" {
			id, err := strconv.Atoi(r[0])
			if err != nil {
				log.Errorln(err)
				return
			}
			//This is an ABSURD way to get the local host name:
			var HostChan chan minicli.Responses
			bob, _ := minicli.CompileCommand(fmt.Sprintf("host name"))
			HostChan = runCommand(bob, false)
			H := <-HostChan
			Host := H[0].Response

			format := `<tr><td>%v</td><td>%v</td><td><a href="/gui/vnc/%v/%v">%v</a></td>`
			tl := fmt.Sprintf(format, id, Host, Host, 5900+id, r[1])
			for _, entry := range r[2:] {
				tl += `<td>` + entry + `</td>`
			}
			tl += `</tr>`
			info = append(info, tl)
		}
	}
	//get mesh response
	for sa := range respAll {
		if len(sa) != 0 {
			for _, node := range sa {
				for _, s := range node.Tabular {
					if s[2] == "ERROR" {
						id, err := strconv.Atoi(s[0])
						if err != nil {
							log.Errorln(err)
							return
						}

						format := `<tr><td>%v</td><td>%v</td><td><a href="/gui/vnc/%v/%v">%v</a></td>`
						tl := fmt.Sprintf(format, id, node.Host, node.Host, 5900+id, s[1])
						for _, entry := range s[2:] {
							tl += `<td>` + entry + `</td>`
						}
						tl += `</tr>`
						info = append(info, tl)
					}
				}
			}
		}
	}
	body := fmt.Sprintf(`<table id="example" class="hover" cellspacing="0"> %s </tbody></table>`, strings.Join(info, "\n"), `<br>insert flush button here<br>insert start button here`)
	tabletype := `<script type="text/javascript" language="javascript" src="/gui/d3/table.js"></script>`
	writer.Write([]byte(fmt.Sprintf(HTMLFRAME, tabletype, body)))
}

func guiTiler(writer http.ResponseWriter, request *http.Request) {
	var resp chan minicli.Responses
	var respAll chan minicli.Responses
	mask := "id,name,state"
	cmdLocal, err := minicli.CompileCommand(".columns " + mask + " vm info")
	if err != nil {
		// Should never happen
		log.Fatalln(err)
	}
	cmdRemote, err := minicli.CompileCommand(fmt.Sprintf(".columns %s mesh send all vm info", mask))
	if err != nil {
		// Should never happen
		log.Fatalln(err)
	}
	resp = runCommand(cmdLocal, false)
	respAll = runCommand(cmdRemote, false)

	format := `<div style="float: left; position: relative; padding-right: 4px; padding-bottom: 3px;"><a href="/gui/vnc/%v/%v"><img src="/gui/screenshot/%v_%v_250.png" alt="%v" /></a></div>`
	info := []string{}
	g := <-resp
	r := g[0].Tabular
	for _, r := range r {
		if r[2] != "ERROR" && r[2] != "QUIT" {
			id, err := strconv.Atoi(r[0])
			if err != nil {
				log.Errorln(err)
				return
			}
			//This is an ABSURD way to get the local host name:
			var HostChan chan minicli.Responses
			bob, _ := minicli.CompileCommand(fmt.Sprintf("host name"))
			HostChan = runCommand(bob, false)
			H := <-HostChan
			Host := H[0].Response

			tl := fmt.Sprintf(format, Host, 5900+id, Host, id, r[1])
			info = append(info, tl)
		}
	}
	//get mesh response
	for sa := range respAll {
		if len(sa) != 0 {
			for _, node := range sa {
				for _, s := range node.Tabular {
					if s[2] != "ERROR" && s[2] != "QUIT" {
						id, err := strconv.Atoi(s[0])
						if err != nil {
							log.Errorln(err)
							return
						}

						tl := fmt.Sprintf(format, node.Host, 5900+id, node.Host, id, s[1])
						info = append(info, tl)
					}
				}
			}
		}
	}
	body := fmt.Sprintf(`<div style="overflow: hidden; margin: 10px;" > %s </div>`, strings.Join(info, "\n"))
	writer.Write([]byte(fmt.Sprintf(HTMLFRAME, "", body)))
}

func guiAllVMs(writer http.ResponseWriter, request *http.Request) {
	var resp chan minicli.Responses
	var respAll chan minicli.Responses
	columnnames := []string{}
	mask := "id,name,state,memory,vcpus,migrate,disk,snapshot,initrd,kernel,cdrom,append,bridge,tap,mac,ip,ip6,vlan,uuid,cc_active,tags"
	//format := `<tr><td><a href="/gui/vnc/%v/%v"><img src="/gui/screenshot/%v_%v_140.png" alt="%v" /></a></td><td>%v</td><td>%v</td><td><a href="/gui/vnc/%v/%v">%v</a></td>`
	format := `<tr><td><a href="/gui/vnc/%v/%v">"/gui/screenshot/%v_%v.png"</a></td><td>%v</td><td>%v</td><td><a href="/gui/vnc/%v/%v">%v</a></td>`
	cmdLocal, err := minicli.CompileCommand(".columns " + mask + " vm info")
	if err != nil {
		// Should never happen
		log.Fatalln(err)
	}
	cmdRemote, err := minicli.CompileCommand(fmt.Sprintf(".columns %s mesh send all vm info", mask))
	if err != nil {
		// Should never happen
		log.Fatalln(err)
	}
	resp = runCommand(cmdLocal, false)
	respAll = runCommand(cmdRemote, false)

	info := []string{}
	g := <-resp
	ga := g[0].Header
	if len(info) == 0 {
		header := `<thead><tr><th>snapshot</th>`
		columnnames = append(columnnames, "snapshot")
		for _, h := range ga {
			header += `<th>` + h + `</th>`
			columnnames = append(columnnames, h)
			if h == "id" {
				header += `<th>` + `host` + `</th>`
				columnnames = append(columnnames, "host")
			}
		}
		header += `</tr></thead><tbody>`
		info = append(info, header)
	}

	bob := g[0].Tabular
	fmt.Println(len(bob))
	for _, r := range bob {
		if r[2] != "ERROR" && r[2] != "QUIT" {
			id, err := strconv.Atoi(r[0])
			if err != nil {
				log.Errorln(err)
				return
			}
			//This is an ABSURD way to get the local host name:
			var HostChan chan minicli.Responses
			bob, _ := minicli.CompileCommand(fmt.Sprintf("host name"))
			HostChan = runCommand(bob, false)
			H := <-HostChan
			Host := H[0].Response

			tl := fmt.Sprintf(format, Host, 5900+id, Host, id, r[1], id, Host, Host, 5900+id, r[1])
			for _, entry := range r[2:] {
				tl += `<td>` + entry + `</td>`
			}
			tl += `</tr>`
			info = append(info, tl)
		}
	}
	//get mesh response
	fmt.Println(len(respAll))
	for sa := range respAll {
		if len(sa) != 0 {
			for _, node := range sa {
				for _, s := range node.Tabular {
					if s[2] != "ERROR" && s[2] != "QUIT" {
						id, err := strconv.Atoi(s[0])
						if err != nil {
							log.Errorln(err)
							return
						}
						tl := fmt.Sprintf(format, node.Host, 5900+id, node.Host, id, s[1], id, node.Host, node.Host, 5900+id, s[1])
						for _, entry := range s[2:] {
							tl += `<td>` + entry + `</td>`
						}
						tl += `</tr>`
						info = append(info, tl)
					}
				}
			}
		}
	}
	columnviz := `<div style="color:#006400"> Toggle Columns: `
	for i, column := range columnnames {
		columnviz = columnviz + fmt.Sprintf(`<a class="toggle-vis" data-column="%v">%v</a>`, i, column)
		if i != len(columnnames) {
			columnviz = columnviz + " | "
		}
	}
	columnviz = columnviz + "</div>"
	body := fmt.Sprintf(`<table id="example" class="hover" cellspacing="0"> %s </tbody></table>`, strings.Join(info, "\n")) + columnviz
	tabletype := `<script type="text/javascript" language="javascript" src="/gui/d3/table.js"></script>`
	writer.Write([]byte(fmt.Sprintf(HTMLFRAME, tabletype, body)))
}
