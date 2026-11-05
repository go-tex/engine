package engine

import "testing"

// ⛔ LES VALEURS VIENNENT DE LA DÉFINITION, pas d'une exécution du code testé.
// tex.web §108: badness(t,s) ≈ 100·(t/s)³, calculé en entiers par r=(t·297)/s puis
// (r³+2¹⁷)/2¹⁸. Un test écrit en relançant la fonction gèlerait ce qu'elle fait, pas
// ce qu'elle doit faire.
func TestBadnessMatchesTheDefinition(t *testing.T) {
	for _, c := range []struct {
		name    string
		t, s, w int
	}{
		{"t=0 est parfait quoi qu'il arrive", 0, 0, 0},
		{"pas d'étirement et un manque: inf_bad", 100, 0, 10000},
		{"étirement négatif: inf_bad aussi", 100, -5, 10000},
		{"t=s: 100·1³ = 100", unity, unity, 100},
		{"t=s/2: 100·(1/2)³ = 12.5 → 12", unity / 2, unity, 12},
		{"t=2s: 100·2³ = 800", 2 * unity, unity, 800},
		// ⛔ 2698, PAS 2700, et l'écart est le sujet: r=(3·65536·297)/65536=891,
		// 891³=707347971, (707347971+2¹⁷)/2¹⁸ = 2698. La forme flottante 100·(t/s)³
		// donne 2700 et serait « plus juste » — c'est précisément pourquoi il ne faut
		// pas l'écrire: TeX décide des coupures serrées avec CES arrondis-là.
		{"t=3s: l'entier donne 2698 là où le flottant dirait 2700", 3 * unity, unity, 2698},
		// r > 1290 ⇒ inf_bad: r = t·297/s, donc t/s > 4.34…
		{"très au-delà: plafonné à inf_bad", 100 * unity, unity, 10000},
	} {
		if got := badness(c.t, c.s); got != c.w {
			t.Errorf("%s: badness(%d,%d) = %d, attendu %d", c.name, c.t, c.s, got, c.w)
		}
	}
}

// ⛔ LA MONOTONIE EST UNE PROPRIÉTÉ DÉMONTRÉE DE LA DÉFINITION (tex.web:2331):
// badness(t+1,s) >= badness(t,s) >= badness(t,s+1). Elle attrape une inversion de
// branche que des valeurs ponctuelles laisseraient passer — notamment aux deux seuils
// de débordement, 7230584 et 1663497, où le calcul change de forme.
func TestBadnessIsMonotone(t *testing.T) {
	for _, s := range []int{1, 297, unity, 1663497, 1663498, 9 * unity} {
		prev := -1
		// ⛔ LA LISTE DOIT ÊTRE CROISSANTE, sinon c'est le test qui ment: une première
		// version plaçait 9·unity (589824) APRÈS 7230585 et accusait badness de
		// décroître alors que c'était t qui reculait. Les deux seuils de débordement,
		// 7230584 et 1663497, sont inclus parce que le calcul y change de forme.
		// 0 < 1 < 1000 < 16384 < 65536 < 131072 < 589824 < 6553600 < 7230584 < 7230585
		for _, tt := range []int{0, 1, 1000, unity / 4, unity, 2 * unity, 9 * unity,
			100 * unity, 7230584, 7230585} {
			b := badness(tt, s)
			if b < prev {
				t.Errorf("badness décroît en t: badness(%d,%d)=%d après %d", tt, s, b, prev)
			}
			prev = b
			if b2 := badness(tt, s+1); b2 > b {
				t.Errorf("badness croît en s: badness(%d,%d)=%d > badness(%d,%d)=%d",
					tt, s+1, b2, tt, s, b)
			}
		}
	}
}

// ⛔⛔ UNE PAGE QUI PORTE DE L'ÉTIREMENT INFINI EST PARFAITE, pas seulement bonne
// (tex.web:19594-19596). C'est le cas quotidien — un \vfill en fin de chapitre — et
// l'ancienne badness, fonction de deux hauteurs seulement, le notait ~10000.
func TestPageWithInfiniteStretchIsPerfect(t *testing.T) {
	p := pageSoFar{total: 100 * unity}
	if got := p.badnessOf(550 * unity); got == 0 {
		t.Fatal("sans étirement, une page à moitié vide ne doit PAS valoir 0")
	}
	p.stretch[1] = unity // 1fil
	if got := p.badnessOf(550 * unity); got != 0 {
		t.Errorf("avec du fil, badness = %v, attendu 0", got)
	}
}

// ⛔ ET LE DÉBORDEMENT SE MESURE CONTRE LE RÉTRÉCISSEMENT, pas contre \vsize
// (tex.web:19598-19599). Refuser toute page plus haute que \vsize, ce que faisait
// l'ancienne version, jette des pages que TeX accepte en resserrant sa colle.
func TestOverfullIsJudgedAgainstShrink(t *testing.T) {
	const goal = 550 * unity
	p := pageSoFar{total: goal + 10*unity}
	if got := p.badnessOf(goal); got < 0x3FFFFFFF {
		t.Error("sans rétrécissement, un dépassement doit être rédhibitoire")
	}
	p.shrink = 20 * unity
	got := p.badnessOf(goal)
	if got >= 0x3FFFFFFF {
		t.Error("un dépassement que la colle peut absorber n'est PAS rédhibitoire")
	}
	if got == 0 {
		t.Error("il n'est pas gratuit non plus: la colle doit travailler")
	}
}
