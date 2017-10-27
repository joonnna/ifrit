while read p; do
      echo $p
      $A="$(cut -d',' -f1 <<<"$p")"
      echo $A
done <node_addrs
