package main

import (
    "os/exec"
    "os"
    "log"
    //"github.com/davecgh/go-spew/spew"
    //"fmt"
    "flag"
    "encoding/json"
    "gopkg.in/AlecAivazis/survey.v1"
    "errors"
    "runtime"
    "strings"
    "github.com/pkg/browser"
)

func main() {
  //// TODO: checks for kubectl and gcloud are accessible on path and gcloud is configured
  // TODO: Add default value for projects list (gcloud config list)

  fullSetupPtr := flag.Bool("c",false,"choose current kube context")
  copyTokenClipboard := flag.Bool("t",false,"copy access token of current context to clipboard")
  serviceLink := flag.String("svc","none", "Give <namespace>:<service_name>:<port> to get proxy link")


  flag.Parse()

    if *serviceLink != "none" {
        err := printServiceLink(*serviceLink)
        checkErr(err)
        os.Exit(0)

    }

  if *copyTokenClipboard == true {
    mcontext,err := getCurrentContext()
    checkErr(err)
    err = tokenToClipboard(mcontext)
    os.Exit(0)
  }

  if *fullSetupPtr == false {
    err := setKubeContextOnly()
    checkErr(err)
    os.Exit(0)
  }


  //Getting projects list
  var allprojects = []string{}
  allprojects = getAllProjects()
  // Choose project
  var qs_project = []*survey.Question{
    {
        Name: "projects",
        Prompt: &survey.Select{
            Message: "Choose a project:",
            Options: allprojects,
            PageSize: len (allprojects),
            //Default: "red",
        },
    },
  }
  answers_project := struct {
    Projects string `survey:"projects"`
  }{}

  err := survey.Ask(qs_project, &answers_project)
  checkErr(err)

  // Set chosen project as active
  _,err = exec.Command("gcloud", "config", "set", "project",string(answers_project.Projects)).Output()
  checkErr(err)

  // Choose cluster
  allclusters := getAllK8s()
  if len(allclusters) == 0 {
    log.Fatal ("ERROR: Chosen project has no kubernetes clusters")
    //os.Exit(1)
  }


  cluster_names := make([]string,0,len(allclusters))
  for k := range allclusters {
    cluster_names = append(cluster_names, k)
  }
  var qs_cluster = []*survey.Question{
    {
        Name: "clusters",
        Prompt: &survey.Select{
            Message: "Choose a cluster:",
            Options: cluster_names,
            PageSize: len(cluster_names),
            //Default: "red",
        },
    },
  }
  answers_cluster := struct {
    Clusters string `survey:"clusters"`
  }{}

  err = survey.Ask(qs_cluster, &answers_cluster)
  checkErr(err)
  // Activate cluster (get kubeconfig credentials for chosen cluster)
  _,err = exec.Command("gcloud", "container", "clusters", "get-credentials",answers_cluster.Clusters,"--zone",allclusters[answers_cluster.Clusters]).Output()

}

func getAllK8s() map[string]string {
  type ClustersList struct {
    Name string `json:"name"`
    Zone string `json:"zone"`
  }
  gclusters,err := exec.Command("gcloud", "container", "clusters","list", "--format=json").Output()
  checkErr(err)
  var clusters []ClustersList
  err = json.Unmarshal(gclusters, &clusters)
  checkErr(err)

  allclusters := make(map[string]string)
  for i:= range clusters {
    allclusters[string(clusters[i].Name)] = string(clusters[i].Zone)
  }
  return allclusters
}

func getAllContexts() []string {
  type ContextDetail struct {
    Name string `json:"name"`
    Context struct {
      Cluster string
      User string
    }
  }
  type Structure struct {
    Contexts []ContextDetail
  }

  //Getting kubeconfig contexts
  kconfig, err := exec.Command("kubectl", "config", "view", "-o=json","--raw=true").Output()
  checkErr(err)
  var contextnames Structure

  err = json.Unmarshal(kconfig,&contextnames)
  checkErr(err)

  var kcontexts = []string{}
  for k:= range contextnames.Contexts {
    kcontexts = append(kcontexts,string(contextnames.Contexts[k].Name))
  }
  return kcontexts
}

func getAllProjects() []string {

  type ProjectsList struct {
    ProjectId string `json:"ProjectId"`
  }

  gprojects,err := exec.Command("gcloud", "projects", "list", "--format=json").Output()
  checkErr(err)
  var projects []ProjectsList
  err = json.Unmarshal(gprojects, &projects)
  checkErr(err)
  var allprojects = []string{}
  for i:= range projects {
    allprojects = append (allprojects,string(projects[i].ProjectId))
  }

  return allprojects
}

func printServiceLink(servicename string) (error) {
    rawsplice := strings.Split(servicename,":")
    link := "http://localhost:8001/api/v1/namespaces/" + rawsplice[0]+ "/services/"+rawsplice[1]+":"+rawsplice[2]+"/proxy"
    browser.OpenURL(link)
    return nil
}

func setKubeContextOnly() (error) {

  var allcontexts = []string{}
  allcontexts = getAllContexts()

  var qs_context = []*survey.Question{
    {
        Name: "contexts",
        Prompt: &survey.Select{
            Message: "Choose a context:",
            Options: allcontexts,
            PageSize: len(allcontexts),
            //Default: "red",
        },
    },
  }
  answers_context := struct {
    Contexts string `survey:"contexts"`
  }{}

  err := survey.Ask(qs_context, &answers_context)
  if err != nil {
    return errors.New("error in context survey")
  }
  _,err = exec.Command("kubectl", "config", "use-context", answers_context.Contexts).Output()
  if err != nil {
    return errors.New("error in kubectl config setup")
  }
  return nil
}

func getCurrentContext() (string, error) {
  mcontext,err := exec.Command("kubectl", "config", "current-context").Output()
  if err != nil {
    return "", errors.New("cannot get current context")
  }
  return strings.TrimSuffix(string(mcontext), "\n"), nil
}

func tokenToClipboard(context string) error {
  arch := runtime.GOOS
  var jsonpath string = "-o=jsonpath={.users[?(@.name=='" + context + "')].user.auth-provider.config.access-token}"
  out, err := exec.Command("kubectl", "config", "view", jsonpath).Output()
  if  err != nil {
      return errors.New("cannot get token")
  }
  toClipboard(out,arch)
  return nil
}

func toClipboard(output []byte, arch string) {
    var copyCmd *exec.Cmd

    // Mac "OS"
    if arch == "darwin" {
        copyCmd = exec.Command("pbcopy")
    }
    // Linux
    if arch == "linux" {
        //copyCmd = exec.Command("xclip", "-selection", "c")
        copyCmd = exec.Command("xclip")
    }

    in, err := copyCmd.StdinPipe()

    if err != nil {
        log.Fatal(err)
    }

    if err := copyCmd.Start(); err != nil {
        log.Fatal(err)
    }

    if _, err := in.Write([]byte(output)); err != nil {
        log.Fatal(err)
    }

    if err := in.Close(); err != nil {
        log.Fatal(err)
    }

    copyCmd.Wait()
}

func checkErr(err error) {
    if err != nil {
         //fmt.Printf("BLABLA: %s",err)
        log.Fatal("ERROR:", err)
    }
}